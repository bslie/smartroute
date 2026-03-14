package decision

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/smartroute/smartroute/internal/domain"
)

// ExplainSnapshot — стабильный снимок для explain (text/JSON).
type ExplainSnapshot struct {
	Destination   string    `json:"destination"`
	IP            string    `json:"ip"`
	State         string    `json:"state"`
	TrafficClass  string    `json:"traffic_class"`
	ClassConf     float64   `json:"class_confidence"`
	ClassSource   string    `json:"class_source"`
	Assignment    string    `json:"assignment"`
	Tunnel        string    `json:"tunnel"`
	Since         time.Time `json:"since"`
	PolicyLevel   int       `json:"policy_level"`
	Reason        string    `json:"reason"`
	Profile       string    `json:"profile,omitempty"`
	Candidates    []CandidateLine `json:"candidates,omitempty"`
	Staleness     string    `json:"staleness,omitempty"`
	StickyCycles  int       `json:"sticky_cycles,omitempty"`
	IsSticky      bool      `json:"is_sticky,omitempty"`
}

// CandidateLine — одна строка кандидата.
type CandidateLine struct {
	Tunnel string  `json:"tunnel"`
	Score  float64 `json:"score,omitempty"`
	Line   string  `json:"line,omitempty"`
	Status string  `json:"status,omitempty"` // "selected" | "excluded" | ""
}

// FormatExplain форматирует снимок в текст (стабильный формат).
func FormatExplain(s *ExplainSnapshot) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Destination: %s (%s)\n", s.IP, s.Destination))
	b.WriteString(fmt.Sprintf("  State: %s", s.State))
	if s.IsSticky {
		b.WriteString(fmt.Sprintf(" (sticky, %d cycles)", s.StickyCycles))
	}
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Traffic class: %s (confidence: %.2f, source: %s)\n", s.TrafficClass, s.ClassConf, s.ClassSource))
	b.WriteString(fmt.Sprintf("  Assignment: tunnel=%s, since %s\n", s.Tunnel, s.Since.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("  Policy level: %d (%s)\n", s.PolicyLevel, s.Reason))
	if s.Profile != "" {
		b.WriteString(fmt.Sprintf("  Profile: %s\n", s.Profile))
	}
	if len(s.Candidates) > 0 {
		b.WriteString("  Candidates:\n")
		for _, c := range s.Candidates {
			b.WriteString("    " + c.Line + "\n")
		}
	}
	if s.Staleness != "" {
		b.WriteString("  Staleness: " + s.Staleness + "\n")
	}
	return b.String()
}

// FormatExplainJSON возвращает JSON.
func FormatExplainJSON(s *ExplainSnapshot) ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

// BuildSnapshot строит ExplainSnapshot из destination и assignment. activeProfile — текущий профиль (default/game).
func BuildSnapshot(d *domain.Destination, now time.Time, activeProfile string) *ExplainSnapshot {
	s := &ExplainSnapshot{
		IP:           d.IP.String(),
		Destination:  d.Domain,
		State:         string(d.State),
		TrafficClass:  string(d.Class),
		ClassConf:     d.DomainConf,
		ClassSource:   "store",
		Staleness:     "observed 0s ago (fresh)",
		Profile:       activeProfile,
	}
	if d.Domain == "" {
		s.Destination = "(no domain)"
	}
	if d.Assignment != nil {
		s.Assignment = d.Assignment.TunnelName
		s.Tunnel = d.Assignment.TunnelName
		s.Since = d.Assignment.CreatedAt
		s.PolicyLevel = d.Assignment.PolicyLevel
		s.Reason = string(d.Assignment.Reason)
		s.StickyCycles = d.Assignment.StickyCount
		s.IsSticky = d.Assignment.IsSticky
		for _, r := range d.Assignment.RejectedWith {
			s.Candidates = append(s.Candidates, CandidateLine{
				Tunnel: r.TunnelName, Score: r.Score,
				Line: fmt.Sprintf("%s: score=%.1f %s", r.TunnelName, r.Score, r.Reason),
			})
		}
		s.Candidates = append(s.Candidates, CandidateLine{
			Tunnel: s.Tunnel, Score: d.Assignment.Score, Status: "selected",
			Line: fmt.Sprintf("%s: score=%.1f SELECTED", s.Tunnel, d.Assignment.Score),
		})
	}
	return s
}
