package eventbus

import (
	"testing"
	"time"

	"github.com/bslie/smartroute/internal/domain"
)

func TestBus_SendAndLast(t *testing.T) {
	bus := New(2, 10)
	bus.Send(domain.Event{
		Type: domain.EventSystemReady, Timestamp: time.Now(),
		Severity: domain.SeverityInfo, Message: "ready",
	})
	evs := bus.Last(5)
	if len(evs) != 1 {
		t.Errorf("Last(5) = %d events", len(evs))
	}
	if evs[0].Type != domain.EventSystemReady {
		t.Errorf("Type = %s", evs[0].Type)
	}
}
