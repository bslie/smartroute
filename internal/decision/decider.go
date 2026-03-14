package decision

import (
	"time"

	"github.com/smartroute/smartroute/internal/domain"
)

// Decider применяет policy stack и возвращает Assignment. Чистая логика.
type Decider struct {
	Classifier *Classifier
	Scorer     *Scorer
	StaticRoutes []domain.StaticRoute
	Tunnels    []*domain.Tunnel
	DefaultTunnel string
	StickyCycles int
	HysteresisWebPct  int
	HysteresisBulkPct int
	HysteresisGamePct int
	StickyBonus int
	ActiveProfile string // "game" | "default" — для game UDP sticky и бонусов
	StickyBonusGame int // бонус для game traffic в game mode (например 200)
}

// Decide выбирает туннель для destination. Возвращает assignment или nil (fallback).
func (d *Decider) Decide(
	dest *domain.Destination,
	latencyByTunnel map[string]int,
	negativeByTunnel map[string]float64,
) *domain.Assignment {
	// Level 1: hard exclude — unavailable/quarantined
	candidates := make([]*domain.Tunnel, 0)
	for _, t := range d.Tunnels {
		if t.Disabled {
			continue
		}
		if t.State == domain.TunnelStateUnavailable || t.State == domain.TunnelStateFailed ||
			t.State == domain.TunnelStateQuarantined {
			continue
		}
		candidates = append(candidates, t)
	}
	if len(candidates) == 0 {
		a := &domain.Assignment{
			DestIP: dest.IP, TunnelName: d.DefaultTunnel, Reason: domain.ReasonFallback,
			PolicyLevel: domain.PolicyLevelFallback, CreatedAt: time.Now(), Generation: 1,
		}
		return a
	}

	// Level 2: static override
	cr := d.Classifier.Classify(dest.IP.String(), dest.Domain, dest.Port)
	if cr.StaticTunnel != "" && cr.Confidence >= 1.0 {
		for _, t := range d.Tunnels {
			if t.Name == cr.StaticTunnel {
				return &domain.Assignment{
					DestIP: dest.IP, TunnelName: cr.StaticTunnel, Reason: domain.ReasonStaticOverride,
					PolicyLevel: domain.PolicyLevelStaticOverride, CreatedAt: time.Now(), Generation: 1,
				}
			}
		}
	}

	// Level 6–7: scoring + hysteresis
	currentTunnel := ""
	if dest.Assignment != nil {
		currentTunnel = dest.Assignment.TunnelName
	}
	var bestTunnel string
	bestScore := -1.0
	rejected := make([]domain.RejectedCandidate, 0)
	for _, t := range candidates {
		lat := latencyByTunnel[t.Name]
		if lat == 0 {
			lat = 999
		}
		neg := negativeByTunnel[t.Name]
		if neg == 0 {
			neg = 1.0
		}
		stickyBonus := d.StickyBonus
		if currentTunnel == t.Name {
			if d.ActiveProfile == "game" && dest.Class == domain.TrafficClassGame {
				if d.StickyBonusGame > 0 {
					stickyBonus = d.StickyBonusGame
				}
			} else {
				stickyBonus = d.StickyBonus
			}
		} else {
			stickyBonus = 0
		}
		score := d.Scorer.Score(lat, &t.Health, neg, currentTunnel == t.Name, stickyBonus)
		if score > bestScore {
			if bestTunnel != "" {
				rejected = append(rejected, domain.RejectedCandidate{TunnelName: bestTunnel, Score: bestScore, Reason: "comparative_scoring"})
			}
			bestScore = score
			bestTunnel = t.Name
		} else {
			rejected = append(rejected, domain.RejectedCandidate{TunnelName: t.Name, Score: score, Reason: "comparative_scoring"})
		}
	}
	if bestTunnel == "" {
		bestTunnel = d.DefaultTunnel
	}
	// Hysteresis: если current assignment sticky и score выше порога — не переключать
	if currentTunnel != "" && dest.Assignment != nil && dest.Assignment.IsSticky {
		curScore := bestScore
		if currentTunnel != bestTunnel {
			for _, t := range candidates {
				if t.Name == currentTunnel {
					lat := latencyByTunnel[t.Name]
					neg := negativeByTunnel[t.Name]
					if neg == 0 {
						neg = 1.0
					}
					curScore = d.Scorer.Score(lat, &t.Health, neg, true, d.StickyBonus)
					break
				}
			}
			threshold := HysteresisThreshold(dest.Class, curScore, d.HysteresisWebPct, d.HysteresisBulkPct, d.HysteresisGamePct)
			if bestScore-curScore <= threshold {
				bestTunnel = currentTunnel
				bestScore = curScore
			}
		}
	}
	return &domain.Assignment{
		DestIP: dest.IP, TunnelName: bestTunnel, Reason: domain.ReasonComparativeScore,
		PolicyLevel: domain.PolicyLevelComparative, Score: bestScore, RejectedWith: rejected,
		CreatedAt: time.Now(), Generation: 1, IsSticky: dest.Assignment != nil && dest.Assignment.TunnelName == bestTunnel && dest.Assignment.StickyCount+1 >= d.StickyCycles,
		StickyCount: stickyCount(dest, bestTunnel, d.StickyCycles),
	}
}

func stickyCount(dest *domain.Destination, tunnel string, need int) int {
	if dest == nil || dest.Assignment == nil || dest.Assignment.TunnelName != tunnel {
		return 1
	}
	c := dest.Assignment.StickyCount + 1
	if c > need {
		return need
	}
	return c
}

// DefaultTunnelFromList возвращает первый default или первый туннель.
func DefaultTunnelFromList(tunnels []*domain.Tunnel) string {
	for _, t := range tunnels {
		if t.IsDefault {
			return t.Name
		}
	}
	if len(tunnels) > 0 {
		return tunnels[0].Name
	}
	return ""
}
