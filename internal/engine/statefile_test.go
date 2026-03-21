package engine

import (
	"net"
	"path/filepath"
	"testing"

	"github.com/bslie/smartroute/internal/domain"
	"github.com/bslie/smartroute/internal/store"
)

func TestWriteReadStateFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st := store.New()
	st.Lock()
	st.Generation = 5
	st.AppliedGen = 5
	st.Ready = true
	st.ActiveProfile = "game"
	st.Tunnels.Set(&domain.Tunnel{Name: "ams", Interface: "wg-ams"})
	st.Destinations.Set(&domain.Destination{
		IP: net.IPv4(8, 8, 8, 8), Domain: "dns.google", State: domain.DestStateAssigned,
	})
	st.Unlock()

	if err := WriteStateFile(st, path); err != nil {
		t.Fatal(err)
	}
	snap, err := ReadStateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Generation != 5 || snap.Applied != 5 || snap.ActiveProfile != "game" {
		t.Errorf("snap = %+v", snap)
	}
	if len(snap.TunnelNames) != 1 || snap.TunnelNames[0] != "ams" {
		t.Errorf("TunnelNames = %v", snap.TunnelNames)
	}
	if len(snap.Destinations) != 1 || snap.Destinations[0].IP != "8.8.8.8" {
		t.Errorf("Destinations = %+v", snap.Destinations)
	}
}

func TestWriteStateFileEmptyPath(t *testing.T) {
	st := store.New()
	if err := WriteStateFile(st, ""); err != nil {
		t.Fatal(err)
	}
}

func TestReadStateFileNotExist(t *testing.T) {
	_, err := ReadStateFile("/nonexistent/path/state.json")
	if err == nil {
		t.Error("expected error")
	}
}
