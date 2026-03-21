package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bslie/smartroute/internal/memlog"
)

func TestWriteMemlogJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memlog.jsonl")
	r := memlog.NewRing(16)
	r.Write("info", "hello")
	r.Write("warn", "world")
	if err := WriteMemlogJSONL(r, path, 10); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 20 {
		t.Fatalf("short file: %q", data)
	}
}
