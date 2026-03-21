package decision

import (
	"net"
	"testing"

	"github.com/bslie/smartroute/internal/domain"
)

func BenchmarkClassifier_Classify(b *testing.B) {
	c := &Classifier{
		StaticRoutes: []domain.StaticRoute{
			{Domain: "example.com", Tunnel: "ams", TrafficClass: "web"},
			{CIDR: "192.168.0.0/16", Tunnel: "msk"},
		},
	}
	ip := net.ParseIP("192.168.1.1").String()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Classify(ip, "", 443)
	}
}
