package decision

import (
	"math"
	"time"

	"github.com/bslie/smartroute/internal/domain"
)

// Scorer вычисляет score для (destination, tunnel). Чистая логика, без os/exec.
type Scorer struct {
	SignalTTL   time.Duration
	StickyBonus int
}

// Score возвращает final_score для пары dest+tunnel.
// base_score = 1000 - latency_ms, затем health_adj, penalty_adj, negative_adj, sticky_adj.
func (s *Scorer) Score(
	latencyMs int,
	tunnelHealth *domain.TunnelHealth,
	negativeFactor float64,
	isCurrentAssignment bool,
	stickyBonus int,
) float64 {
	base := 1000.0 - float64(latencyMs)
	if base < 0 {
		base = 0
	}
	healthAdj := base * tunnelHealth.Score
	penaltyAdj := healthAdj - float64(tunnelHealth.PenaltyMs)
	negAdj := penaltyAdj * negativeFactor
	if isCurrentAssignment && stickyBonus > 0 {
		negAdj += float64(stickyBonus)
	}
	return negAdj
}

// EffectiveConfidence — decay по возрасту сигнала.
func (s *Scorer) EffectiveConfidence(confidence float64, age time.Duration) float64 {
	if s.SignalTTL <= 0 {
		return confidence
	}
	return confidence * math.Exp(-float64(age)/float64(s.SignalTTL))
}

// HysteresisThreshold — порог в % от current score (web=15%, bulk=25%, game=5%).
func HysteresisThreshold(class domain.TrafficClass, currentScore float64, webPct, bulkPct, gamePct int) float64 {
	pct := webPct
	switch class {
	case domain.TrafficClassGame:
		pct = gamePct
	case domain.TrafficClassBulk:
		pct = bulkPct
	}
	return currentScore * float64(pct) / 100
}
