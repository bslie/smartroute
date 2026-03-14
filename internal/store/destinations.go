package store

import (
	"net"
	"sync"

	"github.com/smartroute/smartroute/internal/domain"
)

// DestinationStore — destinations по IP (string key).
type DestinationStore struct {
	mu   sync.RWMutex
	byIP map[string]*domain.Destination
}

// NewDestinationStore создаёт новый store.
func NewDestinationStore() *DestinationStore {
	return &DestinationStore{byIP: make(map[string]*domain.Destination)}
}

// Set добавляет или обновляет destination.
func (s *DestinationStore) Set(d *domain.Destination) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := *d
	if d.Assignment != nil {
		ac := *d.Assignment
		c.Assignment = &ac
	}
	s.byIP[ipKey(d.IP)] = &c
}

// Get возвращает копию по IP.
func (s *DestinationStore) Get(ip net.IP) *domain.Destination {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.byIP[ipKey(ip)]
	if !ok {
		return nil
	}
	c := *d
	if d.Assignment != nil {
		ac := *d.Assignment
		c.Assignment = &ac
	}
	return &c
}

// Delete удаляет по IP.
func (s *DestinationStore) Delete(ip net.IP) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.byIP, ipKey(ip))
}

// All возвращает копии всех.
func (s *DestinationStore) All() []*domain.Destination {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*domain.Destination, 0, len(s.byIP))
	for _, d := range s.byIP {
		c := *d
		if d.Assignment != nil {
			ac := *d.Assignment
			c.Assignment = &ac
		}
		out = append(out, &c)
	}
	return out
}

// Count возвращает количество.
func (s *DestinationStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byIP)
}
