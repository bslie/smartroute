package adapter

import (
	"fmt"
	"net"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/bslie/smartroute/internal/domain"
	"github.com/bslie/smartroute/internal/metrics"
)

// IPRuleState — желаемое/наблюдаемое состояние ip rules (упрощённо: список правил).
type IPRuleState struct {
	Rules []IPRuleEntry
}

// IPRuleEntry — одна запись ip rule.
type IPRuleEntry struct {
	Priority int
	DestCIDR string
	FwMark   uint32
	TableID  int
}

// IPRuleDiff — дифф: add/remove.
type IPRuleDiff struct {
	Add    []IPRuleEntry
	Remove []IPRuleEntry
}

// IPRuleAdapter реализует Reconcilable для ip rule.
type IPRuleAdapter struct {
	priorityBase int
	priorityEnd  int
}

// NewIPRuleAdapter создаёт адаптер.
func NewIPRuleAdapter(priorityBase, priorityEnd int) *IPRuleAdapter {
	return &IPRuleAdapter{priorityBase: priorityBase, priorityEnd: priorityEnd}
}

// Desired возвращает желаемое состояние из cfg и decisions.
func (a *IPRuleAdapter) Desired(cfg interface{}, decisions interface{}) State {
	c, ok := cfg.(*domain.Config)
	if !ok || c == nil {
		return &IPRuleState{Rules: nil}
	}
	decMap := AssignmentsFromDecisions(decisions)
	if decMap == nil {
		return &IPRuleState{Rules: nil}
	}
	tunnelByName := make(map[string]domain.TunnelConfig, len(c.Tunnels))
	tunnelOrder := make(map[string]int, len(c.Tunnels))
	for i, t := range c.Tunnels {
		tunnelByName[t.Name] = t
		tunnelOrder[t.Name] = i + 1
	}
	keys := make([]string, 0, len(decMap))
	for k := range decMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	rules := make([]IPRuleEntry, 0, len(keys))
	priority := a.priorityBase
	for _, k := range keys {
		if priority > a.priorityEnd {
			break
		}
		asg := decMap[k]
		if asg == nil || asg.DestIP == nil || asg.TunnelName == "" {
			continue
		}
		tc, ok := tunnelByName[asg.TunnelName]
		if !ok {
			continue
		}
		fw := tc.FWMark
		if fw == 0 {
			fw = uint32(tunnelOrder[asg.TunnelName])
		}
		table := tc.RouteTable
		if table == 0 {
			table = 200 + tunnelOrder[asg.TunnelName]
		}
		dest := normalizeDest(asg.DestIP)
		if dest == "" {
			continue
		}
		rules = append(rules, IPRuleEntry{
			Priority: priority,
			DestCIDR: dest,
			FwMark:   fw,
			TableID:  table,
		})
		priority++
	}
	return &IPRuleState{Rules: rules}
}

// Observe читает текущие ip rules (только свои приоритеты).
func (a *IPRuleAdapter) Observe() (State, error) {
	out, err := exec.Command("ip", "rule", "show").Output()
	if err != nil {
		return &IPRuleState{}, err
	}
	lines := strings.Split(string(out), "\n")
	rules := make([]IPRuleEntry, 0, len(lines))
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		r, ok := parseIPRuleLine(ln)
		if !ok {
			continue
		}
		if r.Priority < a.priorityBase || r.Priority > a.priorityEnd {
			continue
		}
		rules = append(rules, r)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority < rules[j].Priority
	})
	return &IPRuleState{Rules: rules}, nil
}

// Plan вычисляет дифф.
func (a *IPRuleAdapter) Plan(desired, observed State) Diff {
	d, _ := desired.(*IPRuleState)
	o, _ := observed.(*IPRuleState)
	if d == nil {
		d = &IPRuleState{}
	}
	if o == nil {
		o = &IPRuleState{}
	}
	dm := make(map[string]IPRuleEntry, len(d.Rules))
	om := make(map[string]IPRuleEntry, len(o.Rules))
	for _, r := range d.Rules {
		dm[ruleKey(r)] = r
	}
	for _, r := range o.Rules {
		om[ruleKey(r)] = r
	}
	add := make([]IPRuleEntry, 0)
	remove := make([]IPRuleEntry, 0)
	for k, r := range dm {
		if _, ok := om[k]; !ok {
			add = append(add, r)
		}
	}
	for k, r := range om {
		if _, ok := dm[k]; !ok {
			remove = append(remove, r)
		}
	}
	sort.Slice(remove, func(i, j int) bool { return remove[i].Priority > remove[j].Priority })
	sort.Slice(add, func(i, j int) bool { return add[i].Priority < add[j].Priority })
	return &IPRuleDiff{Add: add, Remove: remove}
}

// Apply применяет дифф, обновляет метрики rule_sync_adds/dels.
func (a *IPRuleAdapter) Apply(diff Diff) error {
	d, ok := diff.(*IPRuleDiff)
	if !ok || d == nil {
		return nil
	}
	for _, r := range d.Remove {
		if err := exec.Command("ip", "rule", "del", "priority", strconv.Itoa(r.Priority)).Run(); err != nil {
			// Правило уже удалено внешней стороной — не критично, счётчик не инкрементируем
			continue
		}
		metrics.IncRuleSyncDel()
	}
	for _, r := range d.Add {
		args := []string{
			"rule", "add",
			"priority", strconv.Itoa(r.Priority),
			"to", r.DestCIDR,
			"fwmark", fmt.Sprintf("0x%x", r.FwMark),
			"table", strconv.Itoa(r.TableID),
		}
		if err := exec.Command("ip", args...).Run(); err != nil {
			return fmt.Errorf("ip rule add prio=%d to=%s fwmark=0x%x table=%d: %w", r.Priority, r.DestCIDR, r.FwMark, r.TableID, err)
		}
		metrics.IncRuleSyncAdd()
	}
	return nil
}

// Verify проверяет желаемое состояние.
func (a *IPRuleAdapter) Verify(desired State) error {
	d, ok := desired.(*IPRuleState)
	if !ok || d == nil {
		return nil
	}
	obs, err := a.Observe()
	if err != nil {
		return err
	}
	o := obs.(*IPRuleState)
	om := make(map[string]struct{}, len(o.Rules))
	for _, r := range o.Rules {
		om[ruleKey(r)] = struct{}{}
	}
	for _, r := range d.Rules {
		if _, ok := om[ruleKey(r)]; !ok {
			return fmt.Errorf("ip rule missing: prio=%d to=%s fwmark=0x%x table=%d", r.Priority, r.DestCIDR, r.FwMark, r.TableID)
		}
	}
	return nil
}

// Cleanup удаляет все свои правила.
func (a *IPRuleAdapter) Cleanup() error {
	obs, err := a.Observe()
	if err != nil {
		return nil
	}
	o := obs.(*IPRuleState)
	sort.Slice(o.Rules, func(i, j int) bool { return o.Rules[i].Priority > o.Rules[j].Priority })
	for _, r := range o.Rules {
		_ = exec.Command("ip", "rule", "del", "priority", strconv.Itoa(r.Priority)).Run()
	}
	return nil
}

func normalizeDest(ip net.IP) string {
	if ip == nil {
		return ""
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String() + "/32"
	}
	return ip.String() + "/128"
}

// normalizeDestCIDR приводит "to" из вывода ip rule show к виду с маской (ip show даёт 1.2.3.4 без /32).
func normalizeDestCIDR(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	if strings.Contains(s, "/") {
		return s
	}
	ip := net.ParseIP(s)
	if ip == nil {
		return s
	}
	if ip.To4() != nil {
		return s + "/32"
	}
	return s + "/128"
}

func ruleKey(r IPRuleEntry) string {
	return fmt.Sprintf("%d|%s|%d|%d", r.Priority, r.DestCIDR, r.FwMark, r.TableID)
}

// parseFwmarkValue парсит значение fwmark из вывода ip rule (0x1, 0x00000001, 1).
func parseFwmarkValue(s string) (uint32, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	if s == "" {
		return 0, false
	}
	// hex
	if v, err := strconv.ParseUint(s, 16, 32); err == nil {
		return uint32(v), true
	}
	// decimal (ip rule show иногда выводит без 0x)
	if v, err := strconv.ParseUint(s, 10, 32); err == nil {
		return uint32(v), true
	}
	return 0, false
}

func parseIPRuleLine(line string) (IPRuleEntry, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return IPRuleEntry{}, false
	}
	prs := strings.TrimSuffix(fields[0], ":")
	prio, err := strconv.Atoi(prs)
	if err != nil {
		return IPRuleEntry{}, false
	}
	r := IPRuleEntry{Priority: prio}
	for i := 1; i < len(fields); i++ {
		f := fields[i]
		switch f {
		case "to":
			if i+1 < len(fields) {
				r.DestCIDR = normalizeDestCIDR(fields[i+1])
				i++
			}
		case "fwmark":
			if i+1 < len(fields) {
				if v, ok := parseFwmarkValue(fields[i+1]); ok {
					r.FwMark = v
				}
				i++
			}
		case "lookup", "table":
			if i+1 < len(fields) {
				t, err := strconv.Atoi(strings.TrimSpace(fields[i+1]))
				if err == nil {
					r.TableID = t
				}
				i++
			}
		default:
			if strings.HasPrefix(f, "fwmark") {
				parts := strings.SplitN(f, "0x", 2)
				if len(parts) == 2 {
					if v, ok := parseFwmarkValue(parts[1]); ok {
						r.FwMark = v
					}
				}
			}
		}
	}
	if r.DestCIDR == "" {
		return IPRuleEntry{}, false
	}
	return r, true
}
