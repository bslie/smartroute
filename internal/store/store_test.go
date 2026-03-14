package store

import (
	"net"
	"testing"
	"time"

	"github.com/smartroute/smartroute/internal/domain"
)

func TestTunnelStore(t *testing.T) {
	s := NewTunnelStore()
	t1 := &domain.Tunnel{Name: "ams", State: domain.TunnelStateActive}
	s.Set(t1)
	got := s.Get("ams")
	if got == nil || got.Name != "ams" {
		t.Errorf("Get = %v", got)
	}
	names := s.Names()
	if len(names) != 1 || names[0] != "ams" {
		t.Errorf("Names = %v", names)
	}
}

func TestDestinationStore(t *testing.T) {
	s := NewDestinationStore()
	ip := net.ParseIP("1.2.3.4")
	d := &domain.Destination{IP: ip, Domain: "x.com", Class: domain.TrafficClassWeb}
	s.Set(d)
	got := s.Get(ip)
	if got == nil || got.Domain != "x.com" {
		t.Errorf("Get = %v", got)
	}
	s.Delete(ip)
	if s.Get(ip) != nil {
		t.Error("expected nil after Delete")
	}
}

func TestAssignmentStore(t *testing.T) {
	s := NewAssignmentStore()
	ip := net.ParseIP("1.2.3.4")
	a := &domain.Assignment{TunnelName: "ams", CreatedAt: time.Now()}
	s.Set(ip, a)
	got := s.Get(ip)
	if got == nil || got.TunnelName != "ams" {
		t.Errorf("Get = %v", got)
	}
	s.Set(ip, nil)
	if s.Get(ip) != nil {
		t.Error("expected nil after Set(nil)")
	}
}
