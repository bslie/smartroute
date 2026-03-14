package store

import (
	"net"
	"sync"

	"github.com/bslie/smartroute/internal/domain"
)

// AssignmentStore — активные назначения (destination IP -> assignment).
// Engine — единственный writer; чтение под RLock.
type AssignmentStore struct {
	mu   sync.RWMutex
	byIP map[string]*domain.Assignment
}

// NewAssignmentStore создаёт новый store.
func NewAssignmentStore() *AssignmentStore {
	return &AssignmentStore{byIP: make(map[string]*domain.Assignment)}
}

// Set устанавливает назначение для IP.
func (s *AssignmentStore) Set(ip net.IP, a *domain.Assignment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a == nil {
		delete(s.byIP, ipKey(ip))
		return
	}
	ac := *a
	s.byIP[ipKey(ip)] = &ac
}

// Get возвращает копию назначения по IP.
func (s *AssignmentStore) Get(ip net.IP) *domain.Assignment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.byIP[ipKey(ip)]
	if !ok {
		return nil
	}
	ac := *a
	return &ac
}

// All возвращает копии всех назначений.
func (s *AssignmentStore) All() map[string]*domain.Assignment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]*domain.Assignment, len(s.byIP))
	for k, a := range s.byIP {
		ac := *a
		out[k] = &ac
	}
	return out
}

// Delete удаляет назначение по IP (GC при stale/expired destinations).
func (s *AssignmentStore) Delete(ip net.IP) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.byIP, ipKey(ip))
}
