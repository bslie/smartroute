package engine

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/smartroute/smartroute/internal/store"
)

// StateSnapshot — снимок для записи в файл (CLI читает без демона).
type StateSnapshot struct {
	Generation     uint64    `json:"generation"`
	Applied        uint64    `json:"applied"`
	Ready          bool      `json:"ready"`
	ActiveProfile  string    `json:"active_profile"`
	TunnelNames    []string  `json:"tunnel_names"`
	DestCount      int       `json:"dest_count"`
	DisabledFeat   []string  `json:"disabled_features,omitempty"`
	At             time.Time `json:"at"`
}

// WriteStateFile записывает снимок store в файл (вызывать из engine после tick).
func WriteStateFile(st *store.Store, path string) error {
	if path == "" {
		return nil
	}
	st.RLock()
	snap := StateSnapshot{
		Generation:     st.Generation,
		Applied:        st.AppliedGen,
		Ready:          st.Ready,
		ActiveProfile:  st.ActiveProfile,
		TunnelNames:    st.Tunnels.Names(),
		DestCount:      st.Destinations.Count(),
		DisabledFeat:   defaultCaps.DisabledFeatures(),
		At:             time.Now(),
	}
	st.RUnlock()
	data, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
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

// WriteStateFileSafe — потокобезопасная запись (один writer).
func WriteStateFileSafe(st *store.Store, path string) {
	if path == "" {
		return
	}
	stateFileMu.Lock()
	defer stateFileMu.Unlock()
	_ = WriteStateFile(st, path)
}
