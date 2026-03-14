package memlog

import (
	"sync/atomic"
	"time"
)

// Entry — одна запись в кольцевом буфере.
type Entry struct {
	Time    time.Time
	Level   string
	Message string
}

// Ring — lock-free кольцевой буфер логов (фиксированный размер).
type Ring struct {
	entries []Entry
	size    int
	idx     uint64
}

// NewRing создаёт буфер на size записей.
func NewRing(size int) *Ring {
	if size <= 0 {
		size = 1024
	}
	return &Ring{
		entries: make([]Entry, size),
		size:    size,
	}
}

// Write добавляет запись.
func (r *Ring) Write(level, message string) {
	i := atomic.AddUint64(&r.idx, 1)
	pos := (i - 1) % uint64(r.size)
	r.entries[pos] = Entry{
		Time:    time.Now(),
		Level:   level,
		Message: message,
	}
}

// LastN возвращает последние n записей (новые первые).
func (r *Ring) LastN(n int) []Entry {
	idx := atomic.LoadUint64(&r.idx)
	if idx == 0 {
		return nil
	}
	if n > r.size {
		n = r.size
	}
	if n > int(idx) {
		n = int(idx)
	}
	out := make([]Entry, n)
	for i := 0; i < n; i++ {
		pos := (idx - 1 - uint64(i) + uint64(r.size)*2) % uint64(r.size)
		out[i] = r.entries[pos]
	}
	return out
}
