package domain

import (
	"testing"
)

func TestTunnelState(t *testing.T) {
	states := []TunnelState{
		TunnelStateDeclared, TunnelStateActive, TunnelStateDegraded,
		TunnelStateQuarantined, TunnelStateUnavailable,
	}
	for _, s := range states {
		if s == "" {
			t.Error("empty state")
		}
	}
}

func TestTunnelHealth(t *testing.T) {
	h := TunnelHealth{Score: 1.0, Liveness: LivenessUp}
	if h.Score != 1.0 {
		t.Errorf("Score want 1.0, got %f", h.Score)
	}
	h.Score = 0.3
	if h.Score != 0.3 {
		t.Errorf("Score want 0.3, got %f", h.Score)
	}
}
