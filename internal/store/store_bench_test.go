package store

import (
	"net"
	"testing"

	"github.com/bslie/smartroute/internal/domain"
)

func BenchmarkAssignmentStore_SetGet(b *testing.B) {
	s := NewAssignmentStore()
	ip := net.IPv4(1, 2, 3, 4)
	a := &domain.Assignment{
		DestIP:     ip,
		TunnelName: "ams",
		Reason:     domain.ReasonFallback,
		Score:      42,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Set(ip, a)
		_ = s.Get(ip)
	}
}

func BenchmarkDestinationStore_SetGet(b *testing.B) {
	s := NewDestinationStore()
	ip := net.IPv4(8, 8, 8, 8)
	d := &domain.Destination{
		IP:    ip,
		State: domain.DestStateAssigned,
		Class: domain.TrafficClassWeb,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Set(d)
		_ = s.Get(ip)
	}
}
