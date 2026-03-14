package adapter

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/bslie/smartroute/internal/domain"
	"github.com/bslie/smartroute/internal/metrics"
)

// TCState — текущее состояние tc (qdisc+classes) на интерфейсе.
type TCState struct {
	// ByIface: iface -> описание qdisc (один из "htb", "cake", "none")
	ByIface map[string]string
}

// TCDiff — разница между desired и observed.
type TCDiff struct {
	// ToFlush: интерфейсы, где надо пересоздать qdisc
	ToFlush []string
	// ToApply: полная конфигурация для интерфейсов из ToFlush
	ToApply map[string]TCIfaceConfig
}

// TCIfaceConfig — конфигурация tc для одного интерфейса.
type TCIfaceConfig struct {
	Mode  string // "htb" | "cake"
	Iface string
	// HTB: классы по fwmark-class-index
	Classes []TCClass
	// CAKE: RTT hint
	CakeRTTMs int
}

// TCClass — один HTB класс (game/web/bulk) с приоритетом и ставкой.
type TCClass struct {
	ClassIndex uint8  // из fwmark bits [8:15]
	Name       string // game/web/bulk
	Priority   int    // 1=game, 2=web, 3=bulk
	Rate       string // например "100mbit"
	Ceil       string // ceil rate
	Prio       int
}

// TCAdapter — управляет tc qdisc/class/filter на туннельных интерфейсах.
type TCAdapter struct {
	ManagedIfaces    []string
	lastApply        time.Time
	minApplyInterval time.Duration
}

// NewTCAdapter создаёт адаптер.
func NewTCAdapter(ifaces []string) *TCAdapter {
	return &TCAdapter{
		ManagedIfaces:    ifaces,
		minApplyInterval: 5 * time.Second, // минимальный интервал между tc reconcile
	}
}

// Desired строит желаемое состояние tc по cfg и decisions.
func (a *TCAdapter) Desired(cfgI interface{}, decisionsI interface{}) State {
	cfg, ok := cfgI.(*domain.Config)
	if !ok || cfg == nil {
		return &TCState{ByIface: make(map[string]string)}
	}

	ifaces := a.ifacesFromConfig(cfg)
	byIface := make(map[string]string, len(ifaces))
	mode := strings.ToLower(cfg.QoS.Mode)
	if mode == "" {
		mode = "htb" // default
	}
	for _, iface := range ifaces {
		byIface[iface] = mode
	}
	return &TCState{ByIface: byIface}
}

// Observe читает текущий qdisc на управляемых интерфейсах.
// Если ManagedIfaces пуст — обнаруживает интерфейсы через "wg show interfaces" (для hot-reload туннелей).
func (a *TCAdapter) Observe() (State, error) {
	ifaces := a.ManagedIfaces
	if len(ifaces) == 0 {
		out, err := exec.Command("wg", "show", "interfaces").Output()
		if err == nil {
			list := make([]string, 0, 16)
			for _, name := range strings.Fields(strings.TrimSpace(string(out))) {
				if name != "" {
					list = append(list, name)
				}
			}
			ifaces = list
		}
	}
	byIface := make(map[string]string, len(ifaces))
	for _, iface := range ifaces {
		out, err := exec.Command("tc", "qdisc", "show", "dev", iface).Output()
		if err != nil {
			byIface[iface] = "none"
			continue
		}
		line := strings.TrimSpace(string(out))
		switch {
		case strings.Contains(line, "htb"):
			byIface[iface] = "htb"
		case strings.Contains(line, "cake"):
			byIface[iface] = "cake"
		default:
			byIface[iface] = "none"
		}
	}
	return &TCState{ByIface: byIface}, nil
}

// Plan вычисляет diff: какие интерфейсы нужно (пере)конфигурировать.
func (a *TCAdapter) Plan(desiredI, observedI State) Diff {
	desired, ok1 := desiredI.(*TCState)
	observed, ok2 := observedI.(*TCState)
	if !ok1 || !ok2 {
		return &TCDiff{}
	}
	diff := &TCDiff{ToApply: make(map[string]TCIfaceConfig)}
	for iface, wantMode := range desired.ByIface {
		gotMode := observed.ByIface[iface]
		if gotMode != wantMode {
			diff.ToFlush = append(diff.ToFlush, iface)
			diff.ToApply[iface] = TCIfaceConfig{
				Mode:  wantMode,
				Iface: iface,
			}
		}
	}
	return diff
}

// Apply применяет diff: flush + recreate qdisc/classes/filters.
func (a *TCAdapter) Apply(diffI Diff) error {
	diff, ok := diffI.(*TCDiff)
	if !ok || len(diff.ToFlush) == 0 {
		return nil
	}
	// Debounce: tc reconcile не чаще minApplyInterval
	if time.Since(a.lastApply) < a.minApplyInterval {
		return nil
	}
	a.lastApply = time.Now()

	var errs []string
	for _, iface := range diff.ToFlush {
		cfg, hasCfg := diff.ToApply[iface]
		if !hasCfg {
			continue
		}
		start := time.Now()
		// Flush
		exec.Command("tc", "qdisc", "del", "dev", iface, "root").Run() //nolint:errcheck

		elapsed := time.Since(start).Milliseconds()
		metrics.SetTCFlushMs(elapsed)
		metrics.IncTCFlush()

		// Recreate
		var err error
		switch cfg.Mode {
		case "cake":
			err = a.applyCake(iface, cfg.CakeRTTMs)
		default:
			err = a.applyHTB(iface, cfg.Classes)
		}
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", iface, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("tc apply errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Verify проверяет, что нужный qdisc присутствует на каждом интерфейсе.
func (a *TCAdapter) Verify(desiredI State) error {
	desired, ok := desiredI.(*TCState)
	if !ok {
		return nil
	}
	var errs []string
	for iface, wantMode := range desired.ByIface {
		out, err := exec.Command("tc", "qdisc", "show", "dev", iface).Output()
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: qdisc show failed: %v", iface, err))
			continue
		}
		line := strings.TrimSpace(string(out))
		switch wantMode {
		case "cake":
			if !strings.Contains(line, "cake") {
				errs = append(errs, fmt.Sprintf("%s: want cake, got: %s", iface, line))
			}
		case "htb":
			if !strings.Contains(line, "htb") {
				errs = append(errs, fmt.Sprintf("%s: want htb, got: %s", iface, line))
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("tc verify: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Cleanup удаляет qdisc на управляемых интерфейсах (при пустом ManagedIfaces — по "wg show interfaces").
func (a *TCAdapter) Cleanup() error {
	ifaces := a.ManagedIfaces
	if len(ifaces) == 0 {
		out, err := exec.Command("wg", "show", "interfaces").Output()
		if err == nil {
			for _, name := range strings.Fields(strings.TrimSpace(string(out))) {
				if name != "" {
					ifaces = append(ifaces, name)
				}
			}
		}
	}
	for _, iface := range ifaces {
		start := time.Now()
		exec.Command("tc", "qdisc", "del", "dev", iface, "root").Run() //nolint:errcheck
		metrics.SetTCFlushMs(time.Since(start).Milliseconds())
		metrics.IncTCFlush()
	}
	return nil
}

// applyCake применяет CAKE qdisc — простой anti-bufferbloat.
func (a *TCAdapter) applyCake(iface string, rttMs int) error {
	args := []string{"qdisc", "add", "dev", iface, "root", "cake"}
	if rttMs > 0 {
		args = append(args, "rtt", fmt.Sprintf("%dms", rttMs))
	}
	out, err := exec.Command("tc", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("tc cake add: %v: %s", err, out)
	}
	return nil
}

// applyHTB применяет HTB root qdisc с классами по fwmark для game/web/bulk.
// Топология:
//   root: 1: HTB default 30
//   1:10 — game  (prio 1, rate 90mbit, ceil 100mbit), fq_codel leaf
//   1:20 — web   (prio 2, rate 50mbit, ceil 100mbit), fq_codel leaf
//   1:30 — bulk  (prio 3, rate 10mbit, ceil 100mbit), fq_codel leaf  <-- default
//
// Filters: u32 match на fwmark bits [8:15] (class index из ComposeMark):
//   game classIndex=1 → 1:10
//   web  classIndex=2 → 1:20
//   bulk classIndex=3 → 1:30
func (a *TCAdapter) applyHTB(iface string, _ []TCClass) error {
	run := func(args ...string) error {
		out, err := exec.Command("tc", args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("tc %s: %v: %s", strings.Join(args[:3], " "), err, out)
		}
		return nil
	}

	// Root HTB qdisc, default → class 30 (bulk)
	if err := run("qdisc", "add", "dev", iface, "root", "handle", "1:", "htb", "default", "30"); err != nil {
		return err
	}
	// Root class (1:1) — весь bandwidth
	if err := run("class", "add", "dev", iface, "parent", "1:", "classid", "1:1", "htb",
		"rate", "1gbit", "ceil", "1gbit"); err != nil {
		return err
	}

	type classSpec struct {
		classid    string
		rate, ceil string
		prio       string
		leaf       string
	}
	classes := []classSpec{
		{"1:10", "900mbit", "1gbit", "1", "10:"},
		{"1:20", "500mbit", "1gbit", "2", "20:"},
		{"1:30", "100mbit", "1gbit", "3", "30:"},
	}
	for _, cl := range classes {
		if err := run("class", "add", "dev", iface, "parent", "1:1", "classid", cl.classid, "htb",
			"rate", cl.rate, "ceil", cl.ceil, "prio", cl.prio); err != nil {
			return err
		}
		// fq_codel leaf qdisc
		if err := run("qdisc", "add", "dev", iface, "parent", cl.classid, "handle", cl.leaf, "fq_codel"); err != nil {
			return err
		}
	}

	// Filters: match skb mark (fwmark) bits [8:15] (class index). nftables выставляет mark, tc классифицирует по нему.
	// match mark — сопоставление по skb->mark, не по полю пакета.
	filterMap := map[int]string{1: "1:10", 2: "1:20", 3: "1:30"}
	for classIdx, flowid := range filterMap {
		val := classIdx << 8
		hexVal := fmt.Sprintf("0x%x", val)
		hexMask := "0xff00"
		if err := run("filter", "add", "dev", iface, "parent", "1:", "protocol", "ip",
			"prio", "1", "handle", fmt.Sprintf("%d:", classIdx), "u32",
			"match", "mark", hexVal, hexMask,
			"flowid", flowid); err != nil {
			_ = err
		}
	}
	return nil
}

// ifacesFromConfig возвращает интерфейсы туннелей из конфига.
func (a *TCAdapter) ifacesFromConfig(cfg *domain.Config) []string {
	if len(a.ManagedIfaces) > 0 {
		return a.ManagedIfaces
	}
	out := make([]string, 0, len(cfg.Tunnels))
	for _, t := range cfg.Tunnels {
		out = append(out, "wg-"+t.Name)
	}
	return out
}
