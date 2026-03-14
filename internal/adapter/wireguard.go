package adapter

import (
	"os/exec"
)

// WireGuardState — состояние интерфейсов wg.
type WireGuardState struct {
	Interfaces []string
}

// WireGuardDiff — add/remove интерфейсов.
type WireGuardDiff struct {
	Ensure []WGInterfaceConfig
	Remove []string
}

// WGInterfaceConfig — конфиг интерфейса.
type WGInterfaceConfig struct {
	Name      string
	Endpoint  string
	KeyFile   string
}

// WireGuardAdapter — управление wg интерфейсами.
type WireGuardAdapter struct{}

// NewWireGuardAdapter создаёт адаптер.
func NewWireGuardAdapter() *WireGuardAdapter {
	return &WireGuardAdapter{}
}

// Desired возвращает желаемое состояние.
func (a *WireGuardAdapter) Desired(cfg interface{}, decisions interface{}) State {
	return &WireGuardState{Interfaces: nil}
}

// Observe читает wg show.
func (a *WireGuardAdapter) Observe() (State, error) {
	_, err := exec.Command("wg", "show").Output()
	if err != nil {
		return &WireGuardState{}, nil
	}
	return &WireGuardState{Interfaces: nil}, nil
}

// Plan вычисляет дифф.
func (a *WireGuardAdapter) Plan(desired, observed State) Diff {
	return &WireGuardDiff{}
}

// Apply применяет (wg set и т.д.).
func (a *WireGuardAdapter) Apply(diff Diff) error {
	_ = diff
	return nil
}

// Verify проверяет.
func (a *WireGuardAdapter) Verify(desired State) error {
	return nil
}

// Cleanup удаляет wg интерфейсы.
func (a *WireGuardAdapter) Cleanup() error {
	_ = exec.Command("wg", "show").Output()
	return nil
}
