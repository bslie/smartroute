package domain

import (
	"net"
	"testing"
	"time"
)

func TestDestState(t *testing.T) {
	states := []DestState{
		DestStateDiscovered, DestStateAssigned, DestStateSticky, DestStateStale,
	}
	for _, s := range states {
		if s == "" {
			t.Error("empty state")
		}
	}
}

func TestDestination(t *testing.T) {
	ip := net.ParseIP("1.2.3.4")
	d := &Destination{
		IP: ip, Port: 443, Proto: 6, Domain: "example.com",
		Class: TrafficClassWeb, State: DestStateAssigned,
		LastSeen: time.Now(),
	}
	if d.IP.String() != "1.2.3.4" {
		t.Errorf("IP = %s", d.IP)
	}
	if d.Class != TrafficClassWeb {
		t.Errorf("Class = %s", d.Class)
	}
}
