package decision

import (
	"testing"
	"time"

	"github.com/bslie/smartroute/internal/domain"
)

func TestScorer_Score(t *testing.T) {
	sc := &Scorer{SignalTTL: 120 * time.Second, StickyBonus: 50}
	health := &domain.TunnelHealth{Score: 1.0, PenaltyMs: 0}

	// base = 1000 - latency; health_adj = base * 1.0; neg_adj = health_adj * 1.0
	got := sc.Score(100, health, 1.0, false, 0)
	want := 900.0
	if got != want {
		t.Errorf("Score(100ms, 1.0, 1.0, false, 0) = %v, want %v", got, want)
	}

	// with sticky bonus
	got = sc.Score(100, health, 1.0, true, 50)
	want = 950.0
	if got != want {
		t.Errorf("Score with sticky = %v, want %v", got, want)
	}

	// negative factor
	got = sc.Score(50, health, 0.5, false, 0)
	// base=950, health_adj=950, neg_adj=475
	if got != 475.0 {
		t.Errorf("Score with neg 0.5 = %v, want 475", got)
	}
}

func TestHysteresisThreshold(t *testing.T) {
	// web 15% of 1000 = 150
	got := HysteresisThreshold(domain.TrafficClassWeb, 1000, 15, 25, 5)
	if got != 150 {
		t.Errorf("Web hysteresis = %v, want 150", got)
	}
	got = HysteresisThreshold(domain.TrafficClassGame, 1000, 15, 25, 5)
	if got != 50 {
		t.Errorf("Game hysteresis = %v, want 50", got)
	}
}
