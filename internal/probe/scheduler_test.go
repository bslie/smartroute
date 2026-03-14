package probe

import (
	"testing"
)

func TestScheduler_Allow(t *testing.T) {
	s := NewScheduler(2, 3)
	if !s.Allow("1.2.3.4") {
		t.Error("first Allow should succeed")
	}
	if !s.Allow("1.2.3.4") {
		t.Error("second Allow should succeed")
	}
	if s.Allow("5.6.7.8") {
		t.Error("third Allow (over maxPerTick) should fail")
	}
	s.ResetTick()
	if !s.Allow("5.6.7.8") {
		t.Error("after ResetTick should allow")
	}
}
