package adapter

// State — абстрактное состояние (каждый adapter свой тип).
type State interface{}

// Diff — абстрактный дифф для применения.
type Diff interface{}

// Reconcilable — контракт адаптера data plane.
type Reconcilable interface {
	Desired(cfg interface{}, decisions interface{}) State
	Observe() (State, error)
	Plan(desired, observed State) Diff
	Apply(diff Diff) error
	Verify(desired State) error
	Cleanup() error
}
