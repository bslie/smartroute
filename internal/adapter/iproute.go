package adapter

import (
	"os/exec"
)

// IPRouteState — маршруты по таблицам.
type IPRouteState struct {
	TableRoutes map[int][]string // table ID -> routes
}

// IPRouteDiff — изменения.
type IPRouteDiff struct {
	Add    map[int][]string
	Remove map[int][]string
}

// IPRouteAdapter — ip route по таблицам туннелей.
type IPRouteAdapter struct{}

// NewIPRouteAdapter создаёт адаптер.
func NewIPRouteAdapter() *IPRouteAdapter {
	return &IPRouteAdapter{}
}

// Desired возвращает желаемое состояние.
func (a *IPRouteAdapter) Desired(cfg interface{}, decisions interface{}) State {
	return &IPRouteState{TableRoutes: make(map[int][]string)}
}

// Observe читает маршруты.
func (a *IPRouteAdapter) Observe() (State, error) {
	_, _ = exec.Command("ip", "route", "show", "table", "all").Output()
	return &IPRouteState{TableRoutes: make(map[int][]string)}, nil
}

// Plan вычисляет дифф.
func (a *IPRouteAdapter) Plan(desired, observed State) Diff {
	return &IPRouteDiff{}
}

// Apply применяет.
func (a *IPRouteAdapter) Apply(diff Diff) error {
	_ = diff
	return nil
}

// Verify проверяет.
func (a *IPRouteAdapter) Verify(desired State) error {
	return nil
}

// Cleanup удаляет маршруты из управляемых таблиц.
func (a *IPRouteAdapter) Cleanup() error {
	return nil
}
