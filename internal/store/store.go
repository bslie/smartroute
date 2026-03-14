package store

import (
	"sync"
	"time"

	"github.com/smartroute/smartroute/internal/domain"
)

// Store — агрегат runtime state. Engine — единственный writer; CLI читает под RLock.
type Store struct {
	mu sync.RWMutex

	Tunnels      *TunnelStore
	Destinations *DestinationStore
	Assignments  *AssignmentStore
	History      *HistoryStore

	ConfigState     *domain.ConfigState
	Generation      uint64 // tick counter
	AppliedGen      uint64
	ConfigGeneration uint64 // увеличивается при каждом успешном reload
	AppliedConfigGen uint64 // последняя применённая config generation
	ActiveProfile   string
	Ready           bool
}

// New создаёт Store с дефолтными подхранилищами.
func New() *Store {
	return &Store{
		Tunnels:      NewTunnelStore(),
		Destinations: NewDestinationStore(),
		Assignments:  NewAssignmentStore(),
		History:      NewHistoryStore(600 * time.Second),
	}
}

// RLock для чтения snapshot (CLI).
func (s *Store) RLock() { s.mu.RLock() }

// RUnlock снимает read lock.
func (s *Store) RUnlock() { s.mu.RUnlock() }

// Lock для записи (только engine).
func (s *Store) Lock() { s.mu.Lock() }

// Unlock снимает write lock.
func (s *Store) Unlock() { s.mu.Unlock() }
