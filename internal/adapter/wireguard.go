package adapter

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/bslie/smartroute/internal/domain"
)

// WireGuardState — состояние интерфейсов wg (имена + конфиги для setconf).
type WireGuardState struct {
	Interfaces []string
	Configs    []WGInterfaceConfig // один в один с Tunnels из конфига
}

// WireGuardDiff — add/remove интерфейсов и применение конфига.
type WireGuardDiff struct {
	Ensure []WGInterfaceConfig
	Remove []string
}

// WGInterfaceConfig — конфиг интерфейса для wg setconf.
type WGInterfaceConfig struct {
	Name          string
	Endpoint      string
	PrivateKeyFile string
	PeerPublicKey  string
}

// WireGuardAdapter — управление wg интерфейсами.
type WireGuardAdapter struct{}

// NewWireGuardAdapter создаёт адаптер.
func NewWireGuardAdapter() *WireGuardAdapter {
	return &WireGuardAdapter{}
}

// Desired возвращает желаемое состояние (имена + конфиги для setconf).
func (a *WireGuardAdapter) Desired(cfg interface{}, _ interface{}) State {
	c, ok := cfg.(*domain.Config)
	if !ok || c == nil {
		return &WireGuardState{Interfaces: nil, Configs: nil}
	}
	ifaces := make([]string, 0, len(c.Tunnels))
	configs := make([]WGInterfaceConfig, 0, len(c.Tunnels))
	for _, t := range c.Tunnels {
		name := "wg-" + t.Name
		ifaces = append(ifaces, name)
		configs = append(configs, WGInterfaceConfig{
			Name:            name,
			Endpoint:        t.Endpoint,
			PrivateKeyFile:  t.PrivateKeyFile,
			PeerPublicKey:   t.PeerPublicKey,
		})
	}
	sort.Strings(ifaces)
	return &WireGuardState{Interfaces: ifaces, Configs: configs}
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

// Plan вычисляет дифф: Ensure — все желаемые интерфейсы с конфигом (создать или обновить setconf), Remove — лишние.
func (a *WireGuardAdapter) Plan(desired, observed State) Diff {
	d, _ := desired.(*WireGuardState)
	o, _ := observed.(*WireGuardState)
	if d == nil {
		d = &WireGuardState{}
	}
	if o == nil {
		o = &WireGuardState{}
	}
	have := make(map[string]struct{}, len(o.Interfaces))
	for _, n := range o.Interfaces {
		have[n] = struct{}{}
	}
	configByName := make(map[string]WGInterfaceConfig, len(d.Configs))
	for _, c := range d.Configs {
		configByName[c.Name] = c
	}
	diff := &WireGuardDiff{Ensure: make([]WGInterfaceConfig, 0, len(d.Interfaces)), Remove: make([]string, 0)}
	for _, n := range d.Interfaces {
		if cfg, ok := configByName[n]; ok {
			diff.Ensure = append(diff.Ensure, cfg)
		}
	}
	for _, n := range o.Interfaces {
		if _, want := configByName[n]; !want && strings.HasPrefix(n, "wg-") {
			diff.Remove = append(diff.Remove, n)
		}
	}
	sort.Slice(diff.Ensure, func(i, j int) bool { return diff.Ensure[i].Name < diff.Ensure[j].Name })
	sort.Strings(diff.Remove)
	return diff
}

// interfaceExists возвращает true, если интерфейс name существует (ip link show dev name).
func interfaceExists(name string) bool {
	err := exec.Command("ip", "link", "show", "dev", name).Run()
	return err == nil
}

// Apply удаляет лишние интерфейсы, создаёт/поднимает нужные и применяет wg setconf (ключ, peer).
func (a *WireGuardAdapter) Apply(diff Diff) error {
	d, ok := diff.(*WireGuardDiff)
	if !ok || d == nil {
		return nil
	}
	for _, name := range d.Remove {
		_ = exec.Command("ip", "link", "del", "dev", name).Run()
	}
	for _, cfg := range d.Ensure {
		if err := exec.Command("ip", "link", "add", "dev", cfg.Name, "type", "wireguard").Run(); err != nil {
			if !interfaceExists(cfg.Name) {
				return fmt.Errorf("ip link add %s: %w", cfg.Name, err)
			}
			// интерфейс уже существует — продолжаем
		}
		if err := exec.Command("ip", "link", "set", "up", "dev", cfg.Name).Run(); err != nil {
			return fmt.Errorf("ip link set up %s: %w", cfg.Name, err)
		}
		body, err := buildWGSetconfContent(cfg)
		if err != nil {
			return fmt.Errorf("wg setconf %s: %w", cfg.Name, err)
		}
		if len(body) == 0 {
			continue
		}
		if !interfaceExists(cfg.Name) {
			return fmt.Errorf("interface %s missing before wg setconf", cfg.Name)
		}
		// Передаём конфиг через временный файл: в фоне stdin может быть недоступен, из‑за чего wg setconf даёт "fopen: No such file or directory".
		tmpDir := os.TempDir()
		if tmpDir == "" {
			tmpDir = "/tmp"
		}
		f, err := os.CreateTemp(tmpDir, "smartroute-wg-*.conf")
		if err != nil {
			return fmt.Errorf("wg setconf %s: temp file: %w", cfg.Name, err)
		}
		tmpPath := f.Name()
		if _, err := f.Write(body); err != nil {
			f.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("wg setconf %s: write temp: %w", cfg.Name, err)
		}
		if err := f.Close(); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("wg setconf %s: close temp: %w", cfg.Name, err)
		}
		_ = os.Chmod(tmpPath, 0600) // ключ в файле — только владелец
		defer os.Remove(tmpPath)
		cmd := exec.Command("wg", "setconf", cfg.Name, tmpPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("wg setconf %s: %w: %s", cfg.Name, err, out)
		}
	}
	return nil
}

// Verify проверяет, что все желаемые интерфейсы присутствуют (wg show interfaces).
func (a *WireGuardAdapter) Verify(desired State) error {
	d, ok := desired.(*WireGuardState)
	if !ok || d == nil || len(d.Interfaces) == 0 {
		return nil
	}
	obs, err := a.Observe()
	if err != nil {
		return err
	}
	o, _ := obs.(*WireGuardState)
	if o == nil {
		return nil
	}
	have := make(map[string]struct{}, len(o.Interfaces))
	for _, n := range o.Interfaces {
		have[n] = struct{}{}
	}
	for _, name := range d.Interfaces {
		if _, ok := have[name]; !ok {
			return fmt.Errorf("wireguard interface missing: %s", name)
		}
	}
	return nil
}

// buildWGSetconfContent формирует конфиг для wg setconf: [Interface] PrivateKey и при наличии — [Peer].
func buildWGSetconfContent(cfg WGInterfaceConfig) ([]byte, error) {
	var b bytes.Buffer
	if cfg.PrivateKeyFile != "" {
		key, err := os.ReadFile(cfg.PrivateKeyFile)
		if err != nil {
			return nil, err
		}
		key = bytes.TrimSpace(key)
		b.WriteString("[Interface]\nPrivateKey = ")
		b.Write(key)
		b.WriteByte('\n')
	}
	if cfg.PeerPublicKey != "" {
		b.WriteString("\n[Peer]\nPublicKey = ")
		b.WriteString(strings.TrimSpace(cfg.PeerPublicKey))
		b.WriteByte('\n')
		if cfg.Endpoint != "" {
			b.WriteString("Endpoint = ")
			b.WriteString(strings.TrimSpace(cfg.Endpoint))
			b.WriteByte('\n')
		}
		b.WriteString("AllowedIPs = 0.0.0.0/0, ::/0\n")
	}
	return b.Bytes(), nil
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
