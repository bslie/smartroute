package store

import (
	"sync"

	"github.com/bslie/smartroute/internal/domain"
)

// TunnelStore — туннели по имени.
type TunnelStore struct {
	mu   sync.RWMutex
	byName map[string]*domain.Tunnel
}

// NewTunnelStore создаёт новый store.
func NewTunnelStore() *TunnelStore {
	return &TunnelStore{byName: make(map[string]*domain.Tunnel)}
}

// Set добавляет или обновляет туннель.
func (s *TunnelStore) Set(t *domain.Tunnel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := *t
	s.byName[t.Name] = &c
}

// Get возвращает копию туннеля по имени.
func (s *TunnelStore) Get(name string) *domain.Tunnel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.byName[name]
	if !ok {
		return nil
	}
	c := *t
	return &c
}

// All возвращает копии всех туннелей.
func (s *TunnelStore) All() []*domain.Tunnel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*domain.Tunnel, 0, len(s.byName))
	for _, t := range s.byName {
		c := *t
		out = append(out, &c)
	}
	return out
}

// Names возвращает имена туннелей.
func (s *TunnelStore) Names() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.byName))
	for n := range s.byName {
		names = append(names, n)
	}
	return names
}

// Delete удаляет туннель по имени (hot-reload: туннель убран из конфига).
func (s *TunnelStore) Delete(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.byName, name)
}
