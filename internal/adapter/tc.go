package adapter

import (
	"os/exec"
	"time"
)

// TCState — состояние tc на интерфейсе.
type TCState struct {
	Iface string
	Qdisc string
}

// TCDiff — изменения.
type TCDiff struct {
	Iface string
	Flush bool
}

// TCAdapter — tc qdisc/class/filter.
type TCAdapter struct {
	ManagedIfaces []string
}

// LastFlushDurationMs — длительность последнего tc flush (для метрик).
var LastFlushDurationMs int64

// NewTCAdapter создаёт адаптер.
func NewTCAdapter(ifaces []string) *TCAdapter {
	return &TCAdapter{ManagedIfaces: ifaces}
}

// Desired возвращает желаемое состояние.
func (a *TCAdapter) Desired(cfg interface{}, decisions interface{}) State {
	return &TCState{}
}

// Observe читает tc.
func (a *TCAdapter) Observe() (State, error) {
	if len(a.ManagedIfaces) == 0 {
		return &TCState{}, nil
	}
	_, _ = exec.Command("tc", "qdisc", "show", "dev", a.ManagedIfaces[0]).Output()
	return &TCState{}, nil
}

// Plan вычисляет дифф.
func (a *TCAdapter) Plan(desired, observed State) Diff {
	return &TCDiff{}
}

// Apply применяет.
func (a *TCAdapter) Apply(diff Diff) error {
	_ = diff
	return nil
}

// Verify проверяет.
func (a *TCAdapter) Verify(desired State) error {
	return nil
}

// Cleanup удаляет qdisc на управляемых интерфейсах. Замеряет длительность flush.
func (a *TCAdapter) Cleanup() error {
	for _, iface := range a.ManagedIfaces {
		start := time.Now()
		_ = exec.Command("tc", "qdisc", "del", "root", "dev", iface).Run()
		LastFlushDurationMs = time.Since(start).Milliseconds()
	}
	return nil
}
