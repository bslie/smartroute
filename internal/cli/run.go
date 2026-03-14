package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
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
