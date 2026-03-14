package engine

import (
	"fmt"
	"sync"
	"time"

	"github.com/smartroute/smartroute/internal/adapter"
	"github.com/smartroute/smartroute/internal/domain"
	"github.com/smartroute/smartroute/internal/store"
)

// Reconciler — выполняет reconcile в порядке зависимостей. Debounce.
type Reconciler struct {
	mu         sync.Mutex
	adapters   []adapter.Reconcilable
	minInterval time.Duration
	lastRun    time.Time
	pending    *pendingReconcile
	onError    func(adapterName, phase string, err error)
}

type pendingReconcile struct {
	cfg     *domain.Config
	st      *store.Store
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
func (r *Reconciler) run(p *pendingReconcile) {
	if p == nil {
		return
	}
	decisions := p.st.Assignments.All()
	for _, rec := range r.adapters {
		name := adapterName(rec)
		desired := rec.Desired(p.cfg, decisions)
		observed, err := rec.Observe()
		if err != nil {
			if r.onError != nil {
				r.onError(name, "observe", err)
			}
			continue
		}
		diff := rec.Plan(desired, observed)
		if err := rec.Apply(diff); err != nil && r.onError != nil {
			r.onError(name, "apply", err)
		}
	}
}

func adapterName(rec adapter.Reconcilable) string {
	return fmt.Sprintf("%T", rec)
}

// RunFullReconcile блокирующий полный цикл (для bootstrap).
func (r *Reconciler) RunFullReconcile(cfg *domain.Config, st *store.Store) {
	r.run(&pendingReconcile{cfg: cfg, st: st})
}
