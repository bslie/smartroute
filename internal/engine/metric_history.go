package engine

import (
	"sync"
	"time"
)

// MetricSample — одна точка для графиков Web UI (без полного списка destinations).
type MetricSample struct {
	At                 time.Time `json:"at"`
	DestCount          int       `json:"dest_count"`
	ReconcileCycles    uint64    `json:"reconcile_cycles_total"`
	ReconcileErrors    uint64    `json:"reconcile_errors_total"`
	ProbeTotal         uint64    `json:"probe_total"`
	ProbeFailed        uint64    `json:"probe_failed_total"`
	AssignmentSwitches uint64    `json:"assignment_switches_total"`
	TunnelDegraded     uint64    `json:"tunnel_degraded_events_total"`
}

// MetricHistory — кольцевой буфер последних сэмплов.
type MetricHistory struct {
	mu      sync.Mutex
	samples []MetricSample
	max     int
}

// NewMetricHistory создаёт буфер на max точек (по умолчанию 512).
func NewMetricHistory(max int) *MetricHistory {
	if max <= 0 {
		max = 512
	}
	if max > 10000 {
		max = 10000
	}
	return &MetricHistory{max: max, samples: make([]MetricSample, 0, max)}
}

// Push добавляет снимок из StateSnapshot.
func (h *MetricHistory) Push(snap *StateSnapshot) {
	if h == nil || snap == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	s := MetricSample{
		At:                 snap.At,
		DestCount:          snap.DestCount,
		ReconcileCycles:    snap.ReconcileCycles,
		ReconcileErrors:    snap.ReconcileErrors,
		ProbeTotal:         snap.ProbeTotal,
		ProbeFailed:        snap.ProbeFailed,
		AssignmentSwitches: snap.AssignmentSwitches,
		TunnelDegraded:     snap.TunnelDegraded,
	}
	if len(h.samples) >= h.max {
		h.samples = append(h.samples[1:], s)
	} else {
		h.samples = append(h.samples, s)
	}
}

// Samples возвращает копию в хронологическом порядке (старые первые).
func (h *MetricHistory) Samples() []MetricSample {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]MetricSample, len(h.samples))
	copy(out, h.samples)
	return out
}
