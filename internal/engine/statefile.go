package engine

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/bslie/smartroute/internal/metrics"
	"github.com/bslie/smartroute/internal/store"
)

// StateSnapshot — снимок для записи в файл (CLI читает без демона).
type StateSnapshot struct {
	Generation        uint64    `json:"generation"`
	Applied           uint64    `json:"applied"`
	ConfigGeneration  uint64    `json:"config_generation"`
	AppliedConfigGen  uint64    `json:"applied_config_gen"`
	Ready             bool      `json:"ready"`
	ActiveProfile     string    `json:"active_profile"`
	TunnelNames       []string  `json:"tunnel_names"`
	DestCount         int       `json:"dest_count"`
	DisabledFeat      []string  `json:"disabled_features,omitempty"`
	At                time.Time `json:"at"`

	// Metrics
	ReconcileCycles    uint64 `json:"reconcile_cycles_total"`
	ReconcileErrors    uint64 `json:"reconcile_errors_total"`
	ProbeTotal         uint64 `json:"probe_total"`
	ProbeFailed        uint64 `json:"probe_failed_total"`
	AssignmentSwitches uint64 `json:"assignment_switches_total"`
	TunnelDegraded     uint64 `json:"tunnel_degraded_events_total"`
	RuleSyncAdds       uint64 `json:"rule_sync_adds"`
	RuleSyncDels       uint64 `json:"rule_sync_dels"`
	TCFlushCount       uint64 `json:"tc_flush_count"`
	TCFlushDurationMs  int64  `json:"tc_flush_duration_ms"`
}

// BuildStateSnapshot строит снимок из store. Вызывающий код должен держать st.Lock().
func BuildStateSnapshot(st *store.Store) StateSnapshot {
	m := metrics.LoadAll()
	return StateSnapshot{
		Generation:         st.Generation,
		Applied:            st.AppliedGen,
		ConfigGeneration:   st.ConfigGeneration,
		AppliedConfigGen:   st.AppliedConfigGen,
		Ready:              st.Ready,
		ActiveProfile:      st.ActiveProfile,
		TunnelNames:        st.Tunnels.Names(),
		DestCount:          st.Destinations.Count(),
		DisabledFeat:       defaultCaps.DisabledFeatures(),
		At:                 time.Now(),
		ReconcileCycles:    m.ReconcileCycles,
		ReconcileErrors:    m.ReconcileErrors,
		ProbeTotal:         m.ProbeTotal,
		ProbeFailed:        m.ProbeFailed,
		AssignmentSwitches: m.AssignmentSwitches,
		TunnelDegraded:     m.TunnelDegraded,
		RuleSyncAdds:       m.RuleSyncAdds,
		RuleSyncDels:       m.RuleSyncDels,
		TCFlushCount:       m.TCFlushCount,
		TCFlushDurationMs:  m.TCFlushDurationMs,
	}
}

// WriteStateFileFromSnapshot записывает готовый снимок в файл (без доступа к store).
func WriteStateFileFromSnapshot(snap *StateSnapshot, path string) error {
	if path == "" {
		return nil
	}
	data, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// WriteStateFile записывает снимок store в файл. Для вызова при уже захваченном Lock используйте BuildStateSnapshot + WriteStateFileFromSnapshot.
func WriteStateFile(st *store.Store, path string) error {
	if path == "" {
		return nil
	}
	st.RLock()
	snap := BuildStateSnapshot(st)
	st.RUnlock()
	return WriteStateFileFromSnapshot(&snap, path)
}

// ReadStateFile читает снимок из файла (для CLI status).
func ReadStateFile(path string) (*StateSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var snap StateSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

var stateFileMu sync.Mutex

// WriteStateFileSafe записывает готовый снимок в файл (потокобезопасно). Не держит блокировку store — снимок должен быть построен под Lock в вызывающем коде.
func WriteStateFileSafe(snap *StateSnapshot, path string) {
	if path == "" {
		return
	}
	stateFileMu.Lock()
	defer stateFileMu.Unlock()
	_ = WriteStateFileFromSnapshot(snap, path)
}
