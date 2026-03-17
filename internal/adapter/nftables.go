package adapter

import (
	"bytes"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/bslie/smartroute/internal/domain"
)

// NFTablesState — состояние таблицы smartroute.
type NFTablesState struct {
	Table string
	Rules []string
	Raw   string
}

// NFTablesDiff — изменения (atomic replace).
type NFTablesDiff struct {
	Content string
}

// NFTablesAdapter — nft table smartroute.
type NFTablesAdapter struct {
	TableName string
}

// NewNFTablesAdapter создаёт адаптер.
func NewNFTablesAdapter(table string) *NFTablesAdapter {
	if table == "" {
		table = "smartroute"
	}
	return &NFTablesAdapter{TableName: table}
}

// Desired возвращает желаемое состояние; class bits [8:15] берутся из ReconcileInput.ClassByIP.
func (a *NFTablesAdapter) Desired(cfg interface{}, decisions interface{}) State {
	c, ok := cfg.(*domain.Config)
	if !ok || c == nil {
		return &NFTablesState{Table: a.TableName, Rules: nil}
	}
	decMap := AssignmentsFromDecisions(decisions)
	if decMap == nil {
		decMap = map[string]*domain.Assignment{}
	}
	var classByIP map[string]uint8
	if ri, ok := decisions.(*ReconcileInput); ok && ri.ClassByIP != nil {
		classByIP = ri.ClassByIP
	}
	tunnelIndex := make(map[string]uint8, len(c.Tunnels))
	for i, t := range c.Tunnels {
		tunnelIndex[t.Name] = uint8(i + 1)
	}
	keys := make([]string, 0, len(decMap))
	for k := range decMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	rules := make([]string, 0, len(keys))
	for _, k := range keys {
		asg := decMap[k]
		if asg == nil || asg.DestIP == nil || asg.TunnelName == "" {
			continue
		}
		idx := tunnelIndex[asg.TunnelName]
		if idx == 0 {
			continue
		}
		classIdx := uint8(0)
		if classByIP != nil {
			classIdx = classByIP[k]
		}
		mark := domain.ComposeMark(idx, classIdx)
		ip := asg.DestIP.To4()
		if ip == nil {
			continue
		}
		rules = append(rules, fmt.Sprintf("    ip daddr %s meta mark set 0x%x", ip.String(), mark))
	}
	content := buildNFTContent(a.TableName, rules)
	return &NFTablesState{Table: a.TableName, Rules: rules, Raw: content}
}

// normalizeNFTRuleLine приводит правило к виду "    ip daddr X meta mark set 0xY" (nft list даёт 0x00000101, мы — 0x101).
func normalizeNFTRuleLine(ln string) string {
	ln = strings.TrimSpace(ln)
	if !strings.HasPrefix(ln, "ip daddr ") || !strings.Contains(ln, "meta mark set") {
		return ""
	}
	idx := strings.Index(ln, "meta mark set ")
	if idx < 0 {
		return "    " + ln
	}
	rest := strings.TrimSpace(ln[idx+len("meta mark set "):])
	if strings.HasPrefix(rest, "0x") || strings.HasPrefix(rest, "0X") {
		if v, err := strconv.ParseUint(rest[2:], 16, 32); err == nil {
			rest = fmt.Sprintf("0x%x", v)
		}
	}
	prefix := strings.TrimSpace(ln[:idx])
	return "    " + prefix + " meta mark set " + rest
}

// Observe читает таблицу.
func (a *NFTablesAdapter) Observe() (State, error) {
	out, err := exec.Command("nft", "list", "table", "ip", a.TableName).Output()
	if err != nil {
		return &NFTablesState{Table: a.TableName}, nil
	}
	raw := string(out)
	lines := strings.Split(raw, "\n")
	rules := make([]string, 0)
	for _, ln := range lines {
		norm := normalizeNFTRuleLine(ln)
		if norm != "" {
			rules = append(rules, norm)
		}
	}
	sort.Strings(rules)
	return &NFTablesState{Table: a.TableName, Rules: rules, Raw: raw}, nil
}

// Plan вычисляет дифф.
func (a *NFTablesAdapter) Plan(desired, observed State) Diff {
	d, _ := desired.(*NFTablesState)
	o, _ := observed.(*NFTablesState)
	if d == nil {
		return &NFTablesDiff{Content: ""}
	}
	if o == nil {
		o = &NFTablesState{}
	}
	if equalStringSlices(d.Rules, o.Rules) {
		return &NFTablesDiff{Content: ""}
	}
	return &NFTablesDiff{Content: d.Raw}
}

// Apply применяет (nft -f - с контентом в stdin).
func (a *NFTablesAdapter) Apply(diff Diff) error {
	d, ok := diff.(*NFTablesDiff)
	if !ok || d.Content == "" {
		return nil
	}
	_ = exec.Command("nft", "delete", "table", "ip", a.TableName).Run()
	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = bytes.NewReader([]byte(d.Content))
	return cmd.Run()
}

// Verify проверяет, что текущие правила nft совпадают с желаемыми.
func (a *NFTablesAdapter) Verify(desired State) error {
	d, ok := desired.(*NFTablesState)
	if !ok || d == nil {
		return nil
	}
	obs, err := a.Observe()
	if err != nil {
		return err
	}
	o, _ := obs.(*NFTablesState)
	if o == nil {
		return nil
	}
	if !equalStringSlices(d.Rules, o.Rules) {
		return fmt.Errorf("nftables table %s: rules differ from desired (%d want, %d have)", a.TableName, len(d.Rules), len(o.Rules))
	}
	return nil
}

// Cleanup удаляет таблицу.
func (a *NFTablesAdapter) Cleanup() error {
	_ = exec.Command("nft", "delete", "table", "ip", a.TableName).Run()
	return nil
}

func buildNFTContent(table string, rules []string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("table ip %s {\n", table))
	b.WriteString("  chain sr_prerouting {\n")
	b.WriteString("    type filter hook prerouting priority mangle; policy accept;\n")
	for _, r := range rules {
		b.WriteString(r)
		b.WriteString("\n")
	}
	b.WriteString("  }\n")
	b.WriteString("}\n")
	return b.String()
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aa := append([]string(nil), a...)
	bb := append([]string(nil), b...)
	sort.Strings(aa)
	sort.Strings(bb)
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}
