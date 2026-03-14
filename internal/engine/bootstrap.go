package engine

import (
	"github.com/bslie/smartroute/internal/domain"
	"github.com/bslie/smartroute/internal/store"
)

// Bootstrap выполняет последовательность запуска по плану (шаги 1–9).
func Bootstrap(
	cfg *domain.Config,
	st *store.Store,
	reconciler *Reconciler,
) error {
	// 1. Parse config + validate — уже сделано при загрузке
	if err := cfg.Validate(); err != nil {
		return err
	}
	// 2. Init empty store — store уже создан
	// 3. Check OS capabilities (в т.ч. DNSLog при dnsmasq_log_path)
	RefreshCapabilities()
	RefreshCapabilitiesFromConfig(cfg)
	// 4. Disable unavailable features — пропускаем
	// 5. Init adapters — передаются снаружи в Reconciler
	// 6. Observe initial — делается в первом reconcile
	// 7. Cleanup stale — делается в reconciler.Cleanup при старте при необходимости
	// 8. Reconcile tunnels (wg, routes)
	reconciler.RunFullReconcile(cfg, st)
	// 9. READY — выставляется в первом tick
	st.Lock()
	st.Ready = false
	st.Unlock()
	return nil
}
