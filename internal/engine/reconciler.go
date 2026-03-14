package engine

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/smartroute/smartroute/internal/adapter"
	"github.com/smartroute/smartroute/internal/domain"
	"github.com/smartroute/smartroute/internal/metrics"
	"github.com/smartroute/smartroute/internal/store"
)

// Reconciler — выполняет reconcile в порядке зависимостей. Debounce.
type Reconciler struct {
	mu          sync.Mutex
	adapters    []adapter.Reconcilable
	minInterval time.Duration
	lastRun     time.Time
	pending     *pendingReconcile
	onError     func(adapterName, phase string, err error)
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
	return &Reconciler{adapters: adapters, minInterval: minInterval}
}

// SetErrorLog задаёт callback для логирования ошибок адаптеров (Observe/Apply).
func (r *Reconciler) SetErrorLog(fn func(adapterName, phase string, err error)) {
	r.onError = fn
}

// TriggerReconcile ставит в очередь reconcile (debounce).
func (r *Reconciler) TriggerReconcile(cfg *domain.Config, st *store.Store) {
	r.mu.Lock()
	r.pending = &pendingReconcile{cfg: cfg, st: st}
	now := time.Now()
	if now.Sub(r.lastRun) < r.minInterval {
		r.mu.Unlock()
		return
	}
	r.lastRun = now
	p := r.pending
	r.pending = nil
	r.mu.Unlock()
	r.run(p)
}

// run выполняет reconcile: для каждого адаптера Desired, Observe, Plan, Apply.
// Dependency order: sysctl independent; wg → route → rule → nft → tc.
// Если wg адаптер возвращает ошибку Apply (интерфейс не поднялся),
// последующие адаптеры route/rule/nft/tc пропускаются.
func (r *Reconciler) run(p *pendingReconcile) {
	if p == nil {
		return
	}
	decisions := p.st.Assignments.All()

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
	metrics.IncReconcileCycles()}

func adapterName(rec adapter.Reconcilable) string {
	return fmt.Sprintf("%T", rec)
}

func isWireGuardAdapter(name string) bool {
	return strings.Contains(name, "WireGuard") || strings.Contains(name, "Wireguard")
}

// RunFullReconcile блокирующий полный цикл (для bootstrap).
func (r *Reconciler) RunFullReconcile(cfg *domain.Config, st *store.Store) {
	r.run(&pendingReconcile{cfg: cfg, st: st})
}
