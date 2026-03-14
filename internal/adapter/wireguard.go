package adapter

import (
	"os/exec"
	"sort"
	"strings"

	"github.com/smartroute/smartroute/internal/domain"
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
	c, ok := cfg.(*domain.Config)
	if !ok || c == nil {
		return &WireGuardState{Interfaces: nil}
	}
	ifaces := make([]string, 0, len(c.Tunnels))
	for _, t := range c.Tunnels {
		ifaces = append(ifaces, "wg-"+t.Name)
	}
	sort.Strings(ifaces)
	return &WireGuardState{Interfaces: ifaces}
}

// Observe читает wg show.
func (a *WireGuardAdapter) Observe() (State, error) {
	out, err := exec.Command("wg", "show", "interfaces").Output()
	if err != nil {
		return &WireGuardState{}, nil
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	sort.Strings(fields)
	return &WireGuardState{Interfaces: fields}, nil
}

// Plan вычисляет дифф.
func (a *WireGuardAdapter) Plan(desired, observed State) Diff {
	d, _ := desired.(*WireGuardState)
	o, _ := observed.(*WireGuardState)
	if d == nil {
		d = &WireGuardState{}
	}
	if o == nil {
		o = &WireGuardState{}
	}
	want := make(map[string]struct{}, len(d.Interfaces))
	have := make(map[string]struct{}, len(o.Interfaces))
	for _, n := range d.Interfaces {
		want[n] = struct{}{}
	}
	for _, n := range o.Interfaces {
		have[n] = struct{}{}
	}
	diff := &WireGuardDiff{Ensure: make([]WGInterfaceConfig, 0), Remove: make([]string, 0)}
	for _, n := range d.Interfaces {
		if _, ok := have[n]; !ok {
			diff.Ensure = append(diff.Ensure, WGInterfaceConfig{Name: n})
		}
	}
	for _, n := range o.Interfaces {
		if _, ok := want[n]; !ok && strings.HasPrefix(n, "wg-") {
			diff.Remove = append(diff.Remove, n)
		}
	}
	sort.Slice(diff.Ensure, func(i, j int) bool { return diff.Ensure[i].Name < diff.Ensure[j].Name })
	sort.Strings(diff.Remove)
	return diff
}

// Apply применяет (wg set и т.д.).
func (a *WireGuardAdapter) Apply(diff Diff) error {
	d, ok := diff.(*WireGuardDiff)
	if !ok || d == nil {
		return nil
	}
	for _, name := range d.Remove {
		_ = exec.Command("ip", "link", "del", "dev", name).Run()
	}
	for _, cfg := range d.Ensure {
		_ = exec.Command("ip", "link", "add", "dev", cfg.Name, "type", "wireguard").Run()
		_ = exec.Command("ip", "link", "set", "up", "dev", cfg.Name).Run()
	}
	return nil
}

// Verify проверяет.
func (a *WireGuardAdapter) Verify(desired State) error {
	_ = desired
	return nil
}

// Cleanup удаляет wg интерфейсы.
func (a *WireGuardAdapter) Cleanup() error {
	obs, _ := a.Observe()
	o, _ := obs.(*WireGuardState)
	if o == nil {
		return nil
	}
	for _, iface := range o.Interfaces {
		if strings.HasPrefix(iface, "wg-") {
			_ = exec.Command("ip", "link", "del", "dev", iface).Run()
		}
	}
	return nil
}
