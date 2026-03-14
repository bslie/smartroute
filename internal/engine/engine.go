package engine

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/smartroute/smartroute/internal/domain"
	"github.com/smartroute/smartroute/internal/eventbus"
	"github.com/smartroute/smartroute/internal/memlog"
	"github.com/smartroute/smartroute/internal/store"
)

// Engine — оркестратор: tick loop, решение, обновление store, отправка desired в reconciler.
type Engine struct {
	Store        *store.Store
	Bus          *eventbus.Bus
	MemLog       *memlog.Ring
	Reconciler   *Reconciler
	ConfigMu     *sync.RWMutex
	Config       **domain.Config
	TickInterval time.Duration
	StateFile    string // путь для дампа состояния (status без демона)
	GameModeFile string // путь к файлу профиля (game/default) — читается каждый тик
	Ready        bool
	cancel       context.CancelFunc
	prevScores   map[string]float64 // tunnel name -> last health score для событий degraded/recovered
}

// New создаёт engine.
func New(
	st *store.Store,
	bus *eventbus.Bus,
	ml *memlog.Ring,
	rec *Reconciler,
	cfgMu *sync.RWMutex,
	cfg **domain.Config,
) *Engine {
	return &Engine{
		Store:       st,
		Bus:         bus,
		MemLog:      ml,
		Reconciler:  rec,
		ConfigMu:    cfgMu,
		Config:      cfg,
		TickInterval: 2 * time.Second,
	}
}

// Run запускает tick loop в горутине. Контекст для остановки.
func (e *Engine) Run(ctx context.Context) {
	ctx, e.cancel = context.WithCancel(ctx)
	ticker := time.NewTicker(e.TickInterval)
	defer ticker.Stop()
	e.MemLog.Write("info", "engine: tick loop started")
	for {
		select {
		case <-ctx.Done():
			e.MemLog.Write("info", "engine: tick loop stopped")
			return
		case <-ticker.C:
			e.tick(ctx)
		}
	}
}

// tick — один цикл: observe (conntrack), classify, decide, update store, trigger reconcile.
func (e *Engine) tick(ctx context.Context) {
	e.ConfigMu.RLock()
	cfg := *e.Config
	e.ConfigMu.RUnlock()
	if cfg == nil {
		return
	}
	e.Store.Lock()
	defer e.Store.Unlock()
	e.Store.Generation++
	// Упрощённо: обновляем только туннели из конфига и готовность (при первом тике или пустом store).
	if len(e.Store.Tunnels.Names()) == 0 && len(cfg.Tunnels) > 0 {
		for _, tc := range cfg.Tunnels {
			t := &domain.Tunnel{
				Name: tc.Name, Endpoint: tc.Endpoint, Interface: "wg-" + tc.Name,
				RouteTable: 200, FWMark: 1, IsDefault: tc.IsDefault,
				State: domain.TunnelStateDeclared,
				Health: domain.TunnelHealth{Score: 1.0, Liveness: domain.LivenessUp},
			}
			if tc.RouteTable != 0 {
				t.RouteTable = tc.RouteTable
			}
			e.Store.Tunnels.Set(t)
		}
	}
	if !e.Store.Ready {
		e.Store.Ready = true
		e.Store.ActiveProfile = readGameModeFile(e.GameModeFile)
		e.Bus.Send(domain.Event{Type: domain.EventSystemReady, Timestamp: time.Now(), Severity: domain.SeverityInfo, Message: "system ready"})
	}
	if e.GameModeFile != "" {
		if p := readGameModeFile(e.GameModeFile); p != "" && p != e.Store.ActiveProfile {
			e.Store.ActiveProfile = p
		}
	}
	// События tunnel_degraded / tunnel_recovered при смене health score
	const degradedThreshold = 0.8
	if e.prevScores == nil {
		e.prevScores = make(map[string]float64)
	}
	for _, t := range e.Store.Tunnels.All() {
		score := t.Health.Score
		prev, ok := e.prevScores[t.Name]
		e.prevScores[t.Name] = score
		if ok {
			if prev >= degradedThreshold && score < degradedThreshold {
				e.Bus.Send(domain.Event{Type: domain.EventTunnelDegraded, Timestamp: time.Now(), Severity: domain.SeverityWarning, Tunnel: t.Name, Message: "health score dropped below threshold"})
			} else if prev < degradedThreshold && score >= degradedThreshold {
				e.Bus.Send(domain.Event{Type: domain.EventTunnelRecovered, Timestamp: time.Now(), Severity: domain.SeverityInfo, Tunnel: t.Name, Message: "health recovered"})
			}
		}
	}
	// Отправка desired state в reconciler (debounced)
	e.Reconciler.TriggerReconcile(cfg, e.Store)
	e.Store.AppliedGen = e.Store.Generation
	snap := BuildStateSnapshot(e.Store)
	WriteStateFileSafe(&snap, e.StateFile)
}

// Stop останавливает tick loop.
func (e *Engine) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
}

func readGameModeFile(path string) string {
	if path == "" {
		return "default"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "default"
	}
	p := strings.TrimSpace(string(data))
	if p == "game" {
		return "game"
	}
	return "default"
}
