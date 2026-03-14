package decision

import (
	"net"
	"testing"
	"time"

	"github.com/smartroute/smartroute/internal/domain"
)

func TestDecider_Decide(t *testing.T) {
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
	a := d.Decide(dest, latency, neg)
	if a == nil {
		t.Fatal("expected assignment")
	}
	if a.TunnelName != "ams" {
		t.Errorf("expected ams, got %s", a.TunnelName)
	}
	if a.Reason != domain.ReasonComparativeScore {
		t.Errorf("reason = %s", a.Reason)
	}
}

func TestDefaultTunnelFromList(t *testing.T) {
	tunnels := []*domain.Tunnel{
		{Name: "a", IsDefault: false},
		{Name: "b", IsDefault: true},
	}
	if DefaultTunnelFromList(tunnels) != "b" {
		t.Error("expected b as default")
	}
	if DefaultTunnelFromList([]*domain.Tunnel{{Name: "x"}}) != "x" {
		t.Error("expected single tunnel")
	}
	if DefaultTunnelFromList(nil) != "" {
		t.Error("expected empty")
	}
}

func TestDecider_StaticOverride(t *testing.T) {
	cl := &Classifier{
		StaticRoutes: []domain.StaticRoute{{Domain: "x.com", Tunnel: "ams"}},
	}
	d := &Decider{
		Classifier: cl, Scorer: &Scorer{},
		Tunnels: []*domain.Tunnel{{Name: "ams", State: domain.TunnelStateActive, Health: domain.TunnelHealth{Score: 1.0}}},
		DefaultTunnel: "ams",
	}
	dest := &domain.Destination{IP: net.ParseIP("1.2.3.4"), Domain: "x.com", Port: 443}
	a := d.Decide(dest, nil, nil)
	if a.TunnelName != "ams" || a.Reason != domain.ReasonStaticOverride {
		t.Errorf("got %s %s", a.TunnelName, a.Reason)
	}
}
