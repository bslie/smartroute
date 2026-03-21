package engine

import (
	"errors"
	"net"
	"sort"
	"time"

	"github.com/bslie/smartroute/internal/domain"
	"github.com/bslie/smartroute/internal/store"
)

// ErrInvalidSnapshotIP — в снимке state некорректный IP.
var ErrInvalidSnapshotIP = errors.New("invalid IP in state snapshot")

// DefaultStateSnapshotMaxDestinations — лимит записей destinations в state.json (защита от раздувания файла).
const DefaultStateSnapshotMaxDestinations = 512

func effectiveMaxDestinations(n int) int {
	if n <= 0 {
		return DefaultStateSnapshotMaxDestinations
	}
	if n > 10000 {
		return 10000
	}
	return n
}

// DestinationRecord — сериализуемая копия domain.Destination для state.json.
type DestinationRecord struct {
	IP         string  `json:"ip"`
	Port       uint16  `json:"port,omitempty"`
	Proto      uint8   `json:"proto,omitempty"`
	Domain     string  `json:"domain,omitempty"`
	DomainConf float64 `json:"domain_conf,omitempty"`
	Class      string  `json:"class,omitempty"`
	State      string  `json:"state,omitempty"`
	LastSeen   string  `json:"last_seen,omitempty"`
	FirstSeen  string  `json:"first_seen,omitempty"`
	Assignment *AssignmentRecord `json:"assignment,omitempty"`
}

// AssignmentRecord — сериализуемая копия domain.Assignment.
type AssignmentRecord struct {
	DestIP       string                   `json:"dest_ip"`
	TunnelName   string                   `json:"tunnel"`
	Reason       string                   `json:"reason,omitempty"`
	PolicyLevel  int                      `json:"policy_level,omitempty"`
	Signals      []SignalRecord           `json:"signals,omitempty"`
	Score        float64                  `json:"score,omitempty"`
	RejectedWith []RejectedCandidateRecord `json:"rejected_with,omitempty"`
	Generation   uint64                   `json:"generation,omitempty"`
	CreatedAt    string                   `json:"created_at,omitempty"`
	StickyCount  int                      `json:"sticky_count,omitempty"`
	IsSticky     bool                     `json:"is_sticky,omitempty"`
}

// RejectedCandidateRecord — отклонённый кандидат.
type RejectedCandidateRecord struct {
	TunnelName string  `json:"tunnel"`
	Score      float64 `json:"score,omitempty"`
	Reason     string  `json:"reason,omitempty"`
}

// SignalRecord — сигнал для explain.
type SignalRecord struct {
	Source     string  `json:"source,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
	LatencyMs  int     `json:"latency_ms,omitempty"`
	ErrorClass string  `json:"error_class,omitempty"`
	At         string  `json:"at,omitempty"`
}

func destinationToRecord(d *domain.Destination) DestinationRecord {
	r := DestinationRecord{
		IP:         d.IP.String(),
		Port:       d.Port,
		Proto:      d.Proto,
		Domain:     d.Domain,
		DomainConf: d.DomainConf,
		Class:      string(d.Class),
		State:      string(d.State),
		LastSeen:   d.LastSeen.UTC().Format(time.RFC3339Nano),
		FirstSeen:  d.FirstSeen.UTC().Format(time.RFC3339Nano),
	}
	if d.Assignment != nil {
		r.Assignment = assignmentToRecord(d.Assignment)
	}
	return r
}

func assignmentToRecord(a *domain.Assignment) *AssignmentRecord {
	if a == nil {
		return nil
	}
	out := &AssignmentRecord{
		DestIP:       a.DestIP.String(),
		TunnelName:   a.TunnelName,
		Reason:       string(a.Reason),
		PolicyLevel:  a.PolicyLevel,
		Score:        a.Score,
		Generation:   a.Generation,
		CreatedAt:    a.CreatedAt.UTC().Format(time.RFC3339Nano),
		StickyCount:  a.StickyCount,
		IsSticky:     a.IsSticky,
	}
	for _, s := range a.Signals {
		out.Signals = append(out.Signals, SignalRecord{
			Source:     s.Source,
			Confidence: s.Confidence,
			LatencyMs:  s.LatencyMs,
			ErrorClass: string(s.ErrorClass),
			At:         s.At.UTC().Format(time.RFC3339Nano),
		})
	}
	for _, rw := range a.RejectedWith {
		out.RejectedWith = append(out.RejectedWith, RejectedCandidateRecord{
			TunnelName: rw.TunnelName,
			Score:      rw.Score,
			Reason:     rw.Reason,
		})
	}
	return out
}

// appendDestinationRecords заполняет срез destinations с стабильной сортировкой по IP и лимитом.
func appendDestinationRecords(st *store.Store, max int) ([]DestinationRecord, bool) {
	all := st.Destinations.All()
	sort.Slice(all, func(i, j int) bool {
		return ipKeyLess(all[i].IP, all[j].IP)
	})
	truncated := len(all) > max
	if len(all) > max {
		all = all[:max]
	}
	out := make([]DestinationRecord, 0, len(all))
	for _, d := range all {
		out = append(out, destinationToRecord(d))
	}
	return out, truncated
}

func ipKeyLess(a, b net.IP) bool {
	as := a.String()
	bs := b.String()
	if len(as) != len(bs) {
		return len(as) < len(bs)
	}
	return as < bs
}

// DomainDestinationFromRecord восстанавливает domain.Destination из снимка (для explain CLI).
func DomainDestinationFromRecord(r *DestinationRecord) (*domain.Destination, error) {
	if r == nil {
		return nil, nil
	}
	ip := net.ParseIP(r.IP)
	if ip == nil {
		return nil, ErrInvalidSnapshotIP
	}
	d := &domain.Destination{
		IP:         ip,
		Port:       r.Port,
		Proto:      r.Proto,
		Domain:     r.Domain,
		DomainConf: r.DomainConf,
		Class:      domain.TrafficClass(r.Class),
		State:      domain.DestState(r.State),
	}
	if r.LastSeen != "" {
		if t, err := time.Parse(time.RFC3339Nano, r.LastSeen); err == nil {
			d.LastSeen = t
		}
	}
	if r.FirstSeen != "" {
		if t, err := time.Parse(time.RFC3339Nano, r.FirstSeen); err == nil {
			d.FirstSeen = t
		}
	}
	if r.Assignment != nil {
		a, err := domainAssignmentFromRecord(r.Assignment)
		if err != nil {
			return nil, err
		}
		d.Assignment = a
	}
	return d, nil
}

func domainAssignmentFromRecord(r *AssignmentRecord) (*domain.Assignment, error) {
	if r == nil {
		return nil, nil
	}
	ip := net.ParseIP(r.DestIP)
	if ip == nil {
		return nil, ErrInvalidSnapshotIP
	}
	a := &domain.Assignment{
		DestIP:       ip,
		TunnelName:   r.TunnelName,
		Reason:       domain.AssignmentReason(r.Reason),
		PolicyLevel:  r.PolicyLevel,
		Score:        r.Score,
		Generation:   r.Generation,
		StickyCount:  r.StickyCount,
		IsSticky:     r.IsSticky,
	}
	if r.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, r.CreatedAt); err == nil {
			a.CreatedAt = t
		}
	}
	for _, s := range r.Signals {
		sig := domain.Signal{
			Source:     s.Source,
			Confidence: s.Confidence,
			LatencyMs:  s.LatencyMs,
			ErrorClass: domain.ErrorClass(s.ErrorClass),
		}
		if s.At != "" {
			if t, err := time.Parse(time.RFC3339Nano, s.At); err == nil {
				sig.At = t
			}
		}
		a.Signals = append(a.Signals, sig)
	}
	for _, rw := range r.RejectedWith {
		a.RejectedWith = append(a.RejectedWith, domain.RejectedCandidate{
			TunnelName: rw.TunnelName,
			Score:      rw.Score,
			Reason:     rw.Reason,
		})
	}
	return a, nil
}

// FindDestinationRecord ищет destination в снимке по IP или домену.
func FindDestinationRecord(snap *StateSnapshot, key string) *DestinationRecord {
	if snap == nil || key == "" {
		return nil
	}
	ip := net.ParseIP(key)
	for i := range snap.Destinations {
		d := &snap.Destinations[i]
		if ip != nil && d.IP == ip.String() {
			return d
		}
		if d.Domain != "" && d.Domain == key {
			return d
		}
	}
	return nil
}
