package decision

import (
	"net"
	"testing"

	"github.com/bslie/smartroute/internal/domain"
)

func BenchmarkDecider_Decide(b *testing.B) {
	cl := &Classifier{StaticRoutes: nil}
	sc := &Scorer{StickyBonus: 50}
	t1 := &domain.Tunnel{
		Name: "ams", State: domain.TunnelStateActive,
		Health: domain.TunnelHealth{Score: 1.0, PenaltyMs: 0},
	}
	t2 := &domain.Tunnel{
		Name: "msk", State: domain.TunnelStateActive,
		Health: domain.TunnelHealth{Score: 0.9, PenaltyMs: 0},
	}
	d := &Decider{
		Classifier: cl, Scorer: sc,
		Tunnels: []*domain.Tunnel{t1, t2},
		DefaultTunnel: "ams",
		StickyCycles: 5, StickyBonus: 50,
		HysteresisWebPct: 15, HysteresisBulkPct: 25, HysteresisGamePct: 5,
	}
	dest := &domain.Destination{
		IP: net.ParseIP("142.250.74.110"), Port: 443, Class: domain.TrafficClassWeb,
	}
	latency := map[string]int{"ams": 50, "msk": 200}
	neg := map[string]float64{"ams": 1.0, "msk": 1.0}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.Decide(dest, latency, neg)
	}
}
