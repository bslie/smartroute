package decision

import (
	"net"
	"testing"
	"time"

	"github.com/smartroute/smartroute/internal/domain"
)

func TestBuildSnapshot(t *testing.T) {
	d := &domain.Destination{
		IP: net.ParseIP("1.2.3.4"), Domain: "example.com", Class: domain.TrafficClassWeb,
		State: domain.DestStateAssigned,
		Assignment: &domain.Assignment{
			TunnelName: "ams", Reason: domain.ReasonComparativeScore,
			PolicyLevel: domain.PolicyLevelComparative, Score: 900,
			CreatedAt: time.Now(), StickyCount: 3, IsSticky: true,
		},
	}
	s := BuildSnapshot(d, time.Now(), "game")
	if s.Tunnel != "ams" || s.Profile != "game" {
		t.Errorf("tunnel=%s profile=%s", s.Tunnel, s.Profile)
	}
	if !s.IsSticky || s.StickyCycles != 3 {
		t.Errorf("sticky=%v cycles=%d", s.IsSticky, s.StickyCycles)
	}
}

func TestFormatExplain(t *testing.T) {
	s := &ExplainSnapshot{
		IP: "1.2.3.4", Destination: "example.com", State: "assigned",
		TrafficClass: "web", ClassConf: 0.8, Tunnel: "ams",
		PolicyLevel: 7, Reason: "comparative_scoring", Profile: "default",
	}
	out := FormatExplain(s)
	if out == "" || len(out) < 10 {
		t.Error("FormatExplain returned empty or too short")
	}
}
