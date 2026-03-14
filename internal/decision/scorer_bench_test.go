package decision

import (
	"testing"
	"time"

	"github.com/smartroute/smartroute/internal/domain"
)

func BenchmarkScorer_Score(b *testing.B) {
	sc := &Scorer{SignalTTL: 120 * time.Second, StickyBonus: 50}
	health := &domain.TunnelHealth{Score: 0.95, PenaltyMs: 0}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sc.Score(50+i%200, health, 1.0, false, 0)
	}
}
