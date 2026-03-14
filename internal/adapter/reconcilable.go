package adapter

import "github.com/bslie/smartroute/internal/domain"

// State — абстрактное состояние (каждый adapter свой тип).
type State interface{}

// Diff — абстрактный дифф для применения.
type Diff interface{}

// ReconcileInput — вход для Desired: назначения и IP→class index для fwmark bits [8:15].
// Собирается в reconciler из store (Assignments + Destinations).
type ReconcileInput struct {
	Assignments map[string]*domain.Assignment
	ClassByIP   map[string]uint8 // IP string -> TrafficClass.Index() для nft/tc
}

// AssignmentsFromDecisions возвращает map назначений из ReconcileInput или старого формата map.
func AssignmentsFromDecisions(decisions interface{}) map[string]*domain.Assignment {
	if ri, ok := decisions.(*ReconcileInput); ok {
		return ri.Assignments
	}
	if m, ok := decisions.(map[string]*domain.Assignment); ok {
		return m
	}
	return nil
}

// Reconcilable — контракт адаптера data plane.
type Reconcilable interface {
	Desired(cfg interface{}, decisions interface{}) State
	Observe() (State, error)
	Plan(desired, observed State) Diff
	Apply(diff Diff) error
	Verify(desired State) error
	Cleanup() error
}
