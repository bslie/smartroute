package adapter

import (
	"testing"

	"github.com/smartroute/smartroute/internal/domain"
)

func TestBuildNFTMarkRules(t *testing.T) {
	out := BuildNFTMarkRules("smartroute", map[string]uint8{"ams": 1}, nil)
	if out == "" || len(out) < 20 {
		t.Error("expected non-empty nft rules")
	}
	if !contains(out, "smartroute") {
		t.Error("expected table name in output")
	}
}

func TestMarkForTunnelClass(t *testing.T) {
	m := MarkForTunnelClass(1, domain.TrafficClassWeb)
	if m != 0x0201 {
		t.Errorf("mark = %x, want 0x0201 (tunnel=1, web=2)", m)
	}
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
