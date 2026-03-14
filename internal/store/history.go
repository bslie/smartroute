package store

import (
	"net"
	"sync"
	"time"

	"github.com/smartroute/smartroute/internal/domain"
)

// DestHistoryRecord — лёгкая запись истории после GC.
type DestHistoryRecord struct {
	IP          net.IP
	Domain      string
	LastTunnel  string
	LastScore   float64
	LastSeen    time.Time
	ProbeSummary string
}

// HistoryStore — история destinations (retain 600s после expired).
type HistoryStore struct {
	mu      sync.RWMutex
	byIP    map[string]*DestHistoryRecord
	retain  time.Duration
}

// NewHistoryStore создаёт store с временем хранения.
func NewHistoryStore(retain time.Duration) *HistoryStore {
	if retain <= 0 {
		retain = 600 * time.Second
	}
	return &HistoryStore{byIP: make(map[string]*DestHistoryRecord), retain: retain}
}

// Set сохраняет запись.
func (s *HistoryStore) Set(r *DestHistoryRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rc := *r
	s.byIP[ipKey(r.IP)] = &rc
}

// Get возвращает по IP.
func (s *HistoryStore) Get(ip net.IP) *DestHistoryRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.byIP[ipKey(ip)]
	if !ok {
		return nil
	}
	rc := *r
	return &rc
}

// GC удаляет устаревшие записи.
func (s *HistoryStore) GC(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := now.Add(-s.retain)
	for k, r := range s.byIP {
		if r.LastSeen.Before(cutoff) {
			delete(s.byIP, k)
		}
	}
}

// FromDestination создаёт запись из domain.Destination и domain.DestHistory.
func FromDestination(d *domain.Destination, h *domain.DestHistory) *DestHistoryRecord {
	if d != nil {
		tunnel := ""
		if d.Assignment != nil {
			tunnel = d.Assignment.TunnelName
		}
		score := 0.0
		if d.Assignment != nil {
			score = d.Assignment.Score
		}
		return &DestHistoryRecord{
			IP: d.IP, Domain: d.Domain, LastTunnel: tunnel, LastScore: score,
			LastSeen: d.LastSeen,
		}
	}
	if h != nil {
		return &DestHistoryRecord{
			IP: h.IP, Domain: h.Domain, LastTunnel: h.LastTunnel, LastScore: h.LastScore,
			LastSeen: h.LastSeen, ProbeSummary: h.ProbeSummary,
		}
	}
	return nil
}
