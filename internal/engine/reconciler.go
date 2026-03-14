package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bslie/smartroute/internal/adapter"
	"github.com/bslie/smartroute/internal/domain"
	"github.com/bslie/smartroute/internal/metrics"
	"github.com/bslie/smartroute/internal/store"
)

// Reconciler — выполняет reconcile в порядке зависимостей. Debounce. Работает асинхронно в своей горутине.
type Reconciler struct {
	mu          sync.Mutex
	adapters    []adapter.Reconcilable
	minInterval time.Duration
	lastRun     time.Time
	pending     *pendingReconcile
	onError     func(adapterName, phase string, err error)
	work        chan struct{} // буфер 1: сигнал горутине выполнить reconcile
}

type pendingReconcile struct {
	cfg *domain.Config
	st  *store.Store
}

// NewReconciler создаёт reconciler с адаптерами в порядке: sysctl, wg, route, rule, nft, tc.
func NewReconciler(adapters []adapter.Reconcilable, minInterval time.Duration) *Reconciler {
	if minInterval <= 0 {
		minInterval = 500 * time.Millisecond
	}
	return &Reconciler{adapters: adapters, minInterval: minInterval, work: make(chan struct{}, 1)}
}

// SetErrorLog задаёт callback для логирования ошибок адаптеров (Observe/Apply).
func (r *Reconciler) SetErrorLog(fn func(adapterName, phase string, err error)) {
	r.onError = fn
}

// TriggerReconcile ставит в очередь reconcile и возвращает управление (горутина reconciler выполнит run с debounce).
func (r *Reconciler) TriggerReconcile(cfg *domain.Config, st *store.Store) {
	r.mu.Lock()
	r.pending = &pendingReconcile{cfg: cfg, st: st}
	select {
	case r.work <- struct{}{}:
	default:
		// уже есть сигнал в очереди; worker возьмёт последний pending
	}
	r.mu.Unlock()
}

// Run запускает горутину reconciler: ждёт сигналов по r.work и выполняет run с debounce.
// Вызывать один раз при старте демона (например go rec.Run(ctx)).
func (r *Reconciler) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.work:
			r.mu.Lock()
			p := r.pending
			r.pending = nil
			now := time.Now()
			if now.Sub(r.lastRun) < r.minInterval && p != nil {
				// debounce: отложить на minInterval
				r.pending = p
				r.mu.Unlock()
				time.Sleep(r.minInterval - now.Sub(r.lastRun))
				r.mu.Lock()
				p = r.pending
				r.pending = nil
				now = time.Now()
			}
			if p != nil {
				r.lastRun = now
			}
			r.mu.Unlock()
			if p != nil {
				r.run(p)
			}
		}
	}
}

// run выполняет reconcile: для каждого адаптера Desired, Observe, Plan, Apply.
// Dependency order: sysctl independent; wg → route → rule → nft → tc.
// Если wg адаптер возвращает ошибку Apply (интерфейс не поднялся),
// последующие адаптеры route/rule/nft/tc пропускаются.
func (r *Reconciler) run(p *pendingReconcile) {
	if p == nil {
		return
	}
	decisions := buildReconcileInput(p.st)

	// Состояние прохода — failed dependency блокирует зависимых.
	// Порядок фиксирован: sysctl(0), wg(1), route(2), rule(3), nft(4), tc(5).
	// sysctl независим — ошибка не блокирует других.
	// wg failed → route/rule/nft/tc skip.
	wgFailed := false

	for i, rec := range r.adapters {
		name := adapterName(rec)
		isDependent := i >= 2 // route, rule, nft, tc зависят от wg

		if isDependent && wgFailed {
			// Зависимость от WG провалена — пропускаем, логируем WARN один раз
			if r.onError != nil {
				r.onError(name, "skip", fmt.Errorf("skipped: wireguard dependency failed"))
			}
			continue
		}

		desired := rec.Desired(p.cfg, decisions)
		observed, err := rec.Observe()
		if err != nil {
			if r.onError != nil {
				r.onError(name, "observe", err)
			}
			metrics.IncReconcileError()
			// WG observe error → mark dependency failed
			if isWireGuardAdapter(name) {
				wgFailed = true
			}
			continue
		}
		diff := rec.Plan(desired, observed)
		if applyErr := rec.Apply(diff); applyErr != nil {
			if r.onError != nil {
				r.onError(name, "apply", applyErr)
			}
			metrics.IncReconcileError()
			if isWireGuardAdapter(name) {
				wgFailed = true
			}
			continue
		}
		if verifyErr := rec.Verify(desired); verifyErr != nil && r.onError != nil {
			r.onError(name, "verify", verifyErr)
			metrics.IncReconcileError()
		}
	}
	metrics.IncReconcileCycles()
}

func adapterName(rec adapter.Reconcilable) string {
	return fmt.Sprintf("%T", rec)
}

func isWireGuardAdapter(name string) bool {
	return strings.Contains(name, "WireGuard") || strings.Contains(name, "Wireguard")
}

// buildReconcileInput собирает Assignments и ClassByIP из store для адаптеров (nft/tc — class в fwmark).
func buildReconcileInput(st *store.Store) *adapter.ReconcileInput {
	dests := st.Destinations.All()
	classByIP := make(map[string]uint8, len(dests))
	for _, d := range dests {
		if d != nil && d.IP != nil {
			idx := d.Class.Index()
			if idx < 0 {
				idx = 0
			}
			if idx > 255 {
				idx = 255
			}
			classByIP[d.IP.String()] = uint8(idx)
		}
	}
	return &adapter.ReconcileInput{
		Assignments: st.Assignments.All(),
		ClassByIP:   classByIP,
	}
}

// RunFullReconcile блокирующий полный цикл (для bootstrap).
func (r *Reconciler) RunFullReconcile(cfg *domain.Config, st *store.Store) {
	r.run(&pendingReconcile{cfg: cfg, st: st})
}
