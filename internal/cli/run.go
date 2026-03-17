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
	"github.com/bslie/smartroute/internal/metrics"
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

// loadOrCreateConfig читает конфиг по пути; если файла нет — создаёт каталог и записывает минимальный конфиг.
func loadOrCreateConfig(path string) (*domain.Config, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		cfg := domain.DefaultConfig()
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("config yaml: %w", err)
		}
		return cfg, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("config: %w", err)
	}
	// Файла нет — создаём минимальный конфиг для работы из коробки.
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}
	cfg := domain.DefaultConfig()
	cfg.ClientSubnet = "10.0.0.0/24"
	cfg.Tunnels = nil
	data, err = yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal default config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[*] Создан конфиг по умолчанию: %s\n", path)
	return cfg, nil
}

// ensureRequiredPackages проверяет conntrack, nftables, tc и при отсутствии устанавливает пакеты.
func ensureRequiredPackages() error {
	conntrackOk := exec.Command("conntrack", "-V").Run() == nil
	nftOk := exec.Command("nft", "list", "tables").Run() == nil
	tcOk := exec.Command("tc", "-Version").Run() == nil
	if conntrackOk && nftOk && tcOk {
		return nil
	}
	var toInstall []string
	if !conntrackOk {
		toInstall = append(toInstall, "conntrack")
	}
	if !nftOk {
		toInstall = append(toInstall, "nftables")
	}
	if !tcOk {
		toInstall = append(toInstall, "iproute2")
	}
	if len(toInstall) == 0 {
		return nil
	}
	fmt.Fprintf(os.Stderr, "[*] Устанавливаю недостающие пакеты для полной работы: %v\n", toInstall)
	script := `
export DEBIAN_FRONTEND=noninteractive
need_sudo=""
[ "$(id -u)" -ne 0 ] && need_sudo="sudo"
install_conntrack() {
  command -v conntrack >/dev/null 2>&1 && return 0
  if command -v apt-get >/dev/null 2>&1; then $need_sudo env DEBIAN_FRONTEND=noninteractive apt-get update -qq; $need_sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y -qq conntrack; fi
  if command -v dnf >/dev/null 2>&1; then $need_sudo dnf install -y -q conntrack-tools; fi
  if command -v yum >/dev/null 2>&1; then $need_sudo yum install -y -q conntrack-tools 2>/dev/null || true; fi
  if command -v apk >/dev/null 2>&1; then $need_sudo apk add --no-cache conntrack-tools; fi
}
install_nftables() {
  command -v nft >/dev/null 2>&1 && return 0
  if command -v apt-get >/dev/null 2>&1; then $need_sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y -qq nftables; fi
  if command -v dnf >/dev/null 2>&1; then $need_sudo dnf install -y -q nftables; fi
  if command -v yum >/dev/null 2>&1; then $need_sudo yum install -y -q nftables 2>/dev/null || true; fi
  if command -v apk >/dev/null 2>&1; then $need_sudo apk add --no-cache nftables; fi
}
install_tc() {
  command -v tc >/dev/null 2>&1 && return 0
  if command -v apt-get >/dev/null 2>&1; then $need_sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y -qq iproute2; fi
  if command -v dnf >/dev/null 2>&1; then $need_sudo dnf install -y -q iproute; fi
  if command -v yum >/dev/null 2>&1; then $need_sudo yum install -y -q iproute 2>/dev/null || true; fi
  if command -v apk >/dev/null 2>&1; then $need_sudo apk add --no-cache iproute2; fi
}
install_conntrack; install_nftables; install_tc
`
	cmd := exec.Command("sh", "-c", script)
	cmd.Stdin = nil
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("установка пакетов не удалась: %w", err)
	}
	engine.RefreshCapabilities()
	return nil
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
	sh := exec.Command("sh", script)
	sh.Stdin = nil
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
	// Создаём рабочий каталог для state-файла и game_mode до любых операций.
	if err := os.MkdirAll(filepath.Dir(runStateFile), 0755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	cfg, err := loadOrCreateConfig(runConfigPath)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	// Установка недостающих пакетов (conntrack, nftables, tc) для полной работы.
	if err := ensureRequiredPackages(); err != nil {
		fmt.Fprintf(os.Stderr, "[!] %v (демон продолжит работу в ограниченном режиме)\n", err)
	}
	engine.RefreshCapabilities()

	// Проверка WireGuard: при отсутствии — попытка установки.
	if len(cfg.Tunnels) > 0 || cfg.WireGuardServer != nil {
		if err := ensureWireGuard(); err != nil {
			return err
		}
	}
	// Полная подготовка WG-сервера (wg0 и peers) перед стартом engine.
	if cfg.WireGuardServer != nil {
		if _, _, changed, err := ensureWGServer(cfg, runConfigPath); err != nil {
			return fmt.Errorf("wireguard_server apply: %w", err)
		} else if changed {
			if err := saveConfig(runConfigPath, cfg); err != nil {
				return fmt.Errorf("wireguard_server save config: %w", err)
			}
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
		msg := adapterName + ": " + phase + ": " + err.Error()
		ml.Write("error", msg)
		metrics.SetLastReconcileError(msg)
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
	go rec.Run(ctx) // reconciler в отдельной горутине (async)
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
