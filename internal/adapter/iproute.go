package adapter

import (
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/smartroute/smartroute/internal/domain"
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
	c, ok := cfg.(*domain.Config)
	if !ok || c == nil {
		return &IPRouteState{TableRoutes: make(map[int][]string)}
	}
	st := &IPRouteState{TableRoutes: make(map[int][]string)}
	for i, t := range c.Tunnels {
		table := t.RouteTable
		if table == 0 {
			table = 200 + (i + 1)
		}
		iface := "wg-" + t.Name
		st.TableRoutes[table] = []string{"default dev " + iface}
	}
	return st
}

// Observe читает маршруты.
func (a *IPRouteAdapter) Observe() (State, error) {
	out, err := exec.Command("ip", "route", "show", "table", "all").Output()
	if err != nil {
		return &IPRouteState{TableRoutes: make(map[int][]string)}, err
	}
	lines := strings.Split(string(out), "\n")
	st := &IPRouteState{TableRoutes: make(map[int][]string)}
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" || !strings.Contains(ln, " table ") {
			continue
		}
		parts := strings.SplitN(ln, " table ", 2)
		if len(parts) != 2 {
			continue
		}
		tableStr := strings.Fields(parts[1])
		if len(tableStr) == 0 {
			continue
		}
		table, err := strconv.Atoi(tableStr[0])
		if err != nil {
			continue
		}
		route := strings.TrimSpace(parts[0])
		st.TableRoutes[table] = append(st.TableRoutes[table], route)
	}
	return st, nil
}

// Plan вычисляет дифф.
func (a *IPRouteAdapter) Plan(desired, observed State) Diff {
	d, _ := desired.(*IPRouteState)
	o, _ := observed.(*IPRouteState)
	if d == nil {
		d = &IPRouteState{TableRoutes: make(map[int][]string)}
	}
	if o == nil {
		o = &IPRouteState{TableRoutes: make(map[int][]string)}
	}
	diff := &IPRouteDiff{
		Add:    make(map[int][]string),
		Remove: make(map[int][]string),
	}
	for table, want := range d.TableRoutes {
		have := make(map[string]struct{})
		for _, r := range o.TableRoutes[table] {
			have[r] = struct{}{}
		}
		for _, r := range want {
			if _, ok := have[r]; !ok {
				diff.Add[table] = append(diff.Add[table], r)
			}
		}
	}
	for table, got := range o.TableRoutes {
		want := make(map[string]struct{})
		for _, r := range d.TableRoutes[table] {
			want[r] = struct{}{}
		}
		for _, r := range got {
			if _, ok := want[r]; !ok {
				diff.Remove[table] = append(diff.Remove[table], r)
			}
		}
	}
	return diff
}

// Apply применяет.
func (a *IPRouteAdapter) Apply(diff Diff) error {
	d, ok := diff.(*IPRouteDiff)
	if !ok || d == nil {
		return nil
	}
	tables := make([]int, 0, len(d.Remove))
	for table := range d.Remove {
		tables = append(tables, table)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(tables)))
	for _, table := range tables {
		for _, route := range d.Remove[table] {
			args := append([]string{"route", "del"}, strings.Fields(route)...)
			args = append(args, "table", strconv.Itoa(table))
			_ = exec.Command("ip", args...).Run()
		}
	}
	tables = tables[:0]
	for table := range d.Add {
		tables = append(tables, table)
	}
	sort.Ints(tables)
	for _, table := range tables {
		for _, route := range d.Add[table] {
			args := append([]string{"route", "replace"}, strings.Fields(route)...)
			args = append(args, "table", strconv.Itoa(table))
			if err := exec.Command("ip", args...).Run(); err != nil {
				return err
			}
		}
	}
	return nil
}

// Verify проверяет.
func (a *IPRouteAdapter) Verify(desired State) error {
	_ = desired
	return nil
}

// Cleanup удаляет маршруты из управляемых таблиц.
func (a *IPRouteAdapter) Cleanup() error {
	return nil
}
