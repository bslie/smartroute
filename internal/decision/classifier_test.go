package decision

import (
	"testing"

	"github.com/bslie/smartroute/internal/domain"
)

func TestClassifier_Classify(t *testing.T) {
	c := &Classifier{
		StaticRoutes: []domain.StaticRoute{
			{Domain: "chat.openai.com", Tunnel: "ams", TrafficClass: "web"},
		},
	}
	r := c.Classify("104.18.32.7", "chat.openai.com", 443)
	if r.StaticTunnel != "ams" {
		t.Errorf("StaticTunnel = %q, want ams", r.StaticTunnel)
	}
	if r.Confidence != ConfidenceStatic {
		t.Errorf("Confidence = %f", r.Confidence)
	}
	r2 := c.Classify("1.2.3.4", "", 443)
	if r2.Source != SourcePort {
		t.Errorf("expected port heuristic for 443, got %s", r2.Source)
	}
	r3 := c.Classify("1.2.3.4", "", 0)
	if r3.Source != SourceIPOnly {
		t.Errorf("expected ip_only, got %s", r3.Source)
	}
}
