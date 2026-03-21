package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/bslie/smartroute/internal/memlog"
)

// memlogJSONLLine — одна строка JSONL для экспорта memlog.
type memlogJSONLLine struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

// WriteMemlogJSONL записывает последние maxLines записей memlog в JSONL (старые сверху).
func WriteMemlogJSONL(ring *memlog.Ring, path string, maxLines int) error {
	if path == "" || ring == nil {
		return nil
	}
	if maxLines <= 0 {
		maxLines = 512
	}
	entries := ring.LastN(maxLines)
	if len(entries) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		line := memlogJSONLLine{
			Time:    e.Time.UTC().Format(time.RFC3339Nano),
			Level:   e.Level,
			Message: e.Message,
		}
		if err := enc.Encode(&line); err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
			return err
		}
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}
