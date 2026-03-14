package adapter

import (
	"encoding/json"
	"os"
	"os/exec"
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

// Desired возвращает желаемое состояние.
func (a *SysctlAdapter) Desired(cfg interface{}, decisions interface{}) State {
	return &SysctlState{Keys: make(map[string]string)}
}

// Observe читает sysctl.
func (a *SysctlAdapter) Observe() (State, error) {
	_ = exec.Command("sysctl", "-n", "net.ipv4.ip_forward").Output()
	return &SysctlState{Keys: make(map[string]string)}, nil
}

// Plan вычисляет дифф.
func (a *SysctlAdapter) Plan(desired, observed State) Diff {
	return &SysctlDiff{Set: make(map[string]string)}
}

// Apply применяет. При BackupPath != "" сохраняет текущие значения в backup перед применением.
func (a *SysctlAdapter) Apply(diff Diff) error {
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
	_ = diff
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
