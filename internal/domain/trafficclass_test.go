package domain

import (
	"testing"
)

func TestTrafficClassIndex(t *testing.T) {
	if TrafficClassGame.Index() != 1 {
		t.Errorf("Game index = %d", TrafficClassGame.Index())
	}
	if TrafficClassWeb.Index() != 2 {
		t.Errorf("Web index = %d", TrafficClassWeb.Index())
	}
	if TrafficClassUnknown.Index() != 0 {
		t.Errorf("Unknown index = %d", TrafficClassUnknown.Index())
	}
}
