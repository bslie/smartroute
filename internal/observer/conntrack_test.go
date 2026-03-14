package observer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseConntrackDst(t *testing.T) {
	line := "ipv4     2 tcp      6 123 ESTABLISHED src=10.0.0.1 dst=192.168.1.1 sport=50000 dport=443"
	ip := parseConntrackDst(line)
	if ip == nil || ip.String() != "192.168.1.1" {
		t.Errorf("parseConntrackDst: got %v", ip)
	}
}

func TestParseConntrackDport(t *testing.T) {
	line := "ipv4     2 tcp      6 123 ESTABLISHED src=10.0.0.1 dst=1.2.3.4 sport=50000 dport=443"
	port := parseConntrackDport(line)
	if port != 443 {
		t.Errorf("dport = %d", port)
	}
}

func TestReadConntrack(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "conntrack")
	content := `ipv4     2 tcp      6 120 ESTABLISHED src=10.0.0.2 dst=93.184.216.34 sport=40000 dport=443
ipv4     2 tcp      6 100 ESTABLISHED src=10.0.0.2 dst=93.184.216.34 sport=40001 dport=443
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err := ReadConntrack(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 unique dst:port, got %d", len(entries))
	}
	if entries[0].DstPort != 443 {
		t.Errorf("dport = %d", entries[0].DstPort)
	}
}
