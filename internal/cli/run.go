package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/bslie/smartroute/internal/adapter"
	"github.com/bslie/smartroute/internal/domain"
	"github.com/bslie/smartroute/internal/engine"
	"github.com/bslie/smartroute/internal/eventbus"
	"github.com/bslie/smartroute/internal/memlog"
	"github.com/bslie/smartroute/internal/store"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	runConfigPath string
	runStateFile  string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Запустить демон SmartRoute",
	RunE:  runRun,
}

func init() {
	runCmd.Flags().StringVarP(&runConfigPath, "config", "c", "/etc/smartroute/config.yaml", "путь к конфигу")
	runCmd.Flags().StringVar(&runStateFile, "state-file", "/var/run/smartroute/state.json", "файл состояния для status")
}

// findInstallWireGuardScript возвращает путь к scripts/install-wireguard.sh или пустую строку.
func findInstallWireGuardScript() string {
	if p := os.Getenv("SMARTROUTE_INSTALL_WG_SCRIPT"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		for _, rel := range []string{
			filepath.Join(dir, "..", "share", "smartroute", "install-wireguard.sh"),
			filepath.Join(dir, "..", "scripts", "install-wireguard.sh"),
		} {
			if p, err := filepath.Abs(rel); err == nil {
				if _, err := os.Stat(p); err == nil {
					return p
				}
			}
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		p := filepath.Join(cwd, "scripts", "install-wireguard.sh")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// ensureWireGuard проверяет наличие wg; при отсутствии запускает скрипт установки.
func ensureWireGuard() error {
	engine.RefreshCapabilities()
	if engine.HasWireGuard() {
		return nil
	}
	script := findInstallWireGuardScript()
	if script == "" {
		return fmt.Errorf("WireGuard не найден. Установите: apt install wireguard wireguard-tools (или dnf/apk). Переменная SMARTROUTE_INSTALL_WG_SCRIPT может указывать на скрипт установки")
	}
	// Запуск скрипта (он сам вызовет sudo при необходимости)
	sh := exec.Command("sh", script)
	sh.Stdout = os.Stdout
	sh.Stderr = os.Stderr
	if err := sh.Run(); err != nil {
		return fmt.Errorf("установка WireGuard не удалась: %w", err)
	}
	engine.RefreshCapabilities()
	if !engine.HasWireGuard() {
		return fmt.Errorf("после установки WireGuard команда wg по-прежнему недоступна. Проверьте PATH или перезапустите терминал")
	}
	return nil
}

func runRun(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(runConfigPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	cfg := domain.DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("yaml: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	// Проверка WireGuard: при отсутствии — попытка установки (невидимо для пользователя).
	if len(cfg.Tunnels) > 0 {
		if err := ensureWireGuard(); err != nil {
			return err
		}
	}

	st := store.New()
	bus := eventbus.New(64, 200)
	ml := memlog.NewRing(2048)

	// Адаптеры в порядке зависимостей: sysctl, wg, route, rule, nft, tc.
	// TCAdapter с пустым ManagedIfaces — список интерфейсов берётся из cfg в Desired (hot-reload туннелей учтён).
	adapters := []adapter.Reconcilable{
		adapter.NewSysctlAdapter(nil),
		adapter.NewWireGuardAdapter(),
		adapter.NewIPRouteAdapter(),
		adapter.NewIPRuleAdapter(100, 199),
		adapter.NewNFTablesAdapter("smartroute"),
		adapter.NewTCAdapter(nil),
	}
	rec := engine.NewReconciler(adapters, 500*time.Millisecond)
	rec.SetErrorLog(func(adapterName, phase string, err error) {
		ml.Write("error", adapterName+": "+phase+": "+err.Error())
	})

	var cfgMu sync.RWMutex
	var configGeneration uint64 = 1
	cfgPtr := cfg
	eng := engine.New(st, bus, ml, rec, &cfgMu, &cfgPtr, &configGeneration)
	eng.TickInterval = time.Duration(cfg.TickIntervalMs) * time.Millisecond
	eng.StateFile = runStateFile
	eng.GameModeFile = "/var/run/smartroute/game_mode"

	if err := engine.Bootstrap(cfg, st, rec); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go rec.Run(ctx)  // reconciler в отдельной горутине (async)
	go eng.Run(ctx)

	// SIGHUP debounce: coalesce 500ms после последнего SIGHUP
	var reloadMu sync.Mutex
	reloadTimer := time.NewTimer(0)
	if !reloadTimer.Stop() {
		<-reloadTimer.C
	}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGHUP)
	for {
		select {
		case sig := <-sigCh:
			if sig == syscall.SIGTERM {
				engine.Shutdown(ctx, eng, bus, cfg.ShutdownCleanup, makeCleanupFn(adapters, cfg.ShutdownCleanup))
				return nil
			}
			if sig == syscall.SIGHUP {
				reloadTimer.Reset(500 * time.Millisecond)
			}
		case <-reloadTimer.C:
			reloadMu.Lock()
			data2, err := os.ReadFile(runConfigPath)
			if err != nil {
				reloadMu.Unlock()
				ml.Write("error", "reload: "+err.Error())
				continue
			}
			newCfg := domain.DefaultConfig()
			if err := yaml.Unmarshal(data2, newCfg); err != nil {
				reloadMu.Unlock()
				ml.Write("error", "reload yaml: "+err.Error())
				bus.Send(domain.Event{Type: domain.EventConfigRejected, Timestamp: time.Now(), Severity: domain.SeverityWarning, Message: "invalid config on reload"})
				continue
			}
			if err := newCfg.Validate(); err != nil {
				reloadMu.Unlock()
				ml.Write("error", "reload validate: "+err.Error())
				bus.Send(domain.Event{Type: domain.EventConfigRejected, Timestamp: time.Now(), Severity: domain.SeverityWarning, Message: "config validation failed: " + err.Error()})
				continue
			}
			if newCfg.ClientSubnet != cfg.ClientSubnet {
				reloadMu.Unlock()
				ml.Write("error", "reload: client_subnet immutable")
				bus.Send(domain.Event{Type: domain.EventConfigRejected, Timestamp: time.Now(), Severity: domain.SeverityWarning, Message: "client_subnet immutable"})
				continue
			}
			cfgMu.Lock()
			cfg = newCfg
			cfgPtr = cfg
			cfgMu.Unlock()
			atomic.AddUint64(&configGeneration, 1)
			engine.RefreshCapabilitiesFromConfig(newCfg)
			// Обновляем TickInterval в engine при hot-reload
			if newCfg.TickIntervalMs > 0 {
				eng.TickInterval = time.Duration(newCfg.TickIntervalMs) * time.Millisecond
			}
			reloadMu.Unlock()
			bus.Send(domain.Event{Type: domain.EventConfigReloaded, Timestamp: time.Now(), Severity: domain.SeverityInfo, Message: "config reloaded"})
			ml.Write("info", "config reloaded")
		}
	}
}

// makeCleanupFn вызывает Cleanup() адаптеров: full — все, rules-only — rule+nft, preserve — нет.
func makeCleanupFn(adapters []adapter.Reconcilable, defaultMode string) func(mode string) {
	return func(mode string) {
		if mode == "" {
			mode = defaultMode
		}
		if mode == "preserve" {
			return
		}
		// full: все адаптеры в обратном порядке (tc, nft, rule, route, wg, sysctl не трогаем)
		for i := len(adapters) - 1; i >= 0; i-- {
			_ = adapters[i].Cleanup()
		}
	}
}
