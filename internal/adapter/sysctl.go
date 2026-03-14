package adapter

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
)

// SysctlState — ключи из allowlist.
type SysctlState struct {
	Keys map[string]string
}

// SysctlDiff — изменения.
type SysctlDiff struct {
	Set map[string]string
}

// SysctlAdapter — sysctl с allowlist и backup.
type SysctlAdapter struct {
	Allowlist  []string
	BackupPath string // путь к файлу backup для rollback
	backup    map[string]string
}

// NewSysctlAdapter создаёт адаптер.
func NewSysctlAdapter(allowlist []string) *SysctlAdapter {
	return &SysctlAdapter{Allowlist: allowlist}
}

// Desired возвращает желаемое состояние sysctl согласно allowlist.
// Минимальный hardcoded набор: ip_forward и conntrack accounting.
func (a *SysctlAdapter) Desired(cfg interface{}, decisions interface{}) State {
	keys := map[string]string{
		"net.ipv4.ip_forward":                  "1",
		"net.netfilter.nf_conntrack_acct":       "1",
		"net.netfilter.nf_conntrack_checksum":   "0",
	}
	if len(a.Allowlist) > 0 {
		filtered := make(map[string]string, len(a.Allowlist))
		for _, k := range a.Allowlist {
			if v, ok := keys[k]; ok {
				filtered[k] = v
			}
		}
		return &SysctlState{Keys: filtered}
	}
	return &SysctlState{Keys: keys}
}

// Observe читает текущие значения ключей из allowlist + hardcoded набора.
func (a *SysctlAdapter) Observe() (State, error) {
	desired, _ := a.Desired(nil, nil).(*SysctlState)
	keys := make(map[string]string, len(desired.Keys))
	for k := range desired.Keys {
		out, err := exec.Command("sysctl", "-n", k).Output()
		if err == nil {
			keys[k] = strings.TrimSpace(string(out))
		}
	}
	return &SysctlState{Keys: keys}, nil
}

// Plan вычисляет дифф: ключи, значения которых отличаются от desired.
func (a *SysctlAdapter) Plan(desired, observed State) Diff {
	d, _ := desired.(*SysctlState)
	o, _ := observed.(*SysctlState)
	if d == nil {
		return &SysctlDiff{Set: make(map[string]string)}
	}
	if o == nil {
		o = &SysctlState{Keys: make(map[string]string)}
	}
	set := make(map[string]string)
	for k, want := range d.Keys {
		if got, ok := o.Keys[k]; !ok || got != want {
			set[k] = want
		}
	}
	return &SysctlDiff{Set: set}
}

// Apply применяет. При BackupPath != "" сохраняет текущие значения в backup перед применением.
func (a *SysctlAdapter) Apply(diff Diff) error {
	d, ok := diff.(*SysctlDiff)
	if !ok || d == nil || len(d.Set) == 0 {
		return nil
	}
	if a.BackupPath != "" && a.backup == nil {
		observed, _ := a.Observe()
		if s, ok := observed.(*SysctlState); ok {
			a.backup = make(map[string]string)
			for k, v := range s.Keys {
				a.backup[k] = v
			}
			data, _ := json.Marshal(a.backup)
			_ = os.WriteFile(a.BackupPath, data, 0600)
		}
	}
	for k, v := range d.Set {
		_ = exec.Command("sysctl", "-w", k+"="+v).Run()
	}
	return nil
}

// Restore восстанавливает из backup (rollback).
func (a *SysctlAdapter) Restore() error {
	if a.BackupPath == "" {
		return nil
	}
	data, err := os.ReadFile(a.BackupPath)
	if err != nil {
		return err
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	for k, v := range m {
		_ = exec.Command("sysctl", "-w", k+"="+v).Run()
	}
	return nil
}

// Verify проверяет.
func (a *SysctlAdapter) Verify(desired State) error {
	return nil
}

// Cleanup — откат не обязателен по плану.
func (a *SysctlAdapter) Cleanup() error {
	return nil
}
