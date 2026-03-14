// Package metrics содержит глобальные атомарные счётчики SmartRoute.
// Используется engine, adapter; читается CLI командой metrics.
package metrics

import "sync/atomic"

var (
	ReconcileCyclesTotal uint64
	ReconcileErrorsTotal uint64
	ProbeTotal           uint64
	ProbeFailed          uint64
	AssignmentSwitches   uint64
	TunnelDegradedEvents uint64
	RuleSyncAdds         uint64
	RuleSyncDels         uint64
	TCFlushCount         uint64
	TCFlushDurationMs    int64
)

func IncReconcileCycles()   { atomic.AddUint64(&ReconcileCyclesTotal, 1) }
func IncReconcileError()    { atomic.AddUint64(&ReconcileErrorsTotal, 1) }
func IncProbe()             { atomic.AddUint64(&ProbeTotal, 1) }
func IncProbeFailed()       { atomic.AddUint64(&ProbeFailed, 1) }
func IncAssignmentSwitches() { atomic.AddUint64(&AssignmentSwitches, 1) }
func IncTunnelDegraded()    { atomic.AddUint64(&TunnelDegradedEvents, 1) }
func IncRuleSyncAdd()       { atomic.AddUint64(&RuleSyncAdds, 1) }
func IncRuleSyncDel()       { atomic.AddUint64(&RuleSyncDels, 1) }
func IncTCFlush()           { atomic.AddUint64(&TCFlushCount, 1) }
func SetTCFlushMs(ms int64) { atomic.StoreInt64(&TCFlushDurationMs, ms) }

func LoadRuleSyncAdds() uint64 { return atomic.LoadUint64(&RuleSyncAdds) }
func LoadRuleSyncDels() uint64 { return atomic.LoadUint64(&RuleSyncDels) }
func LoadTCFlushCount() uint64 { return atomic.LoadUint64(&TCFlushCount) }
func LoadTCFlushMs() int64     { return atomic.LoadInt64(&TCFlushDurationMs) }
func LoadAll() Snapshot {
	return Snapshot{
		ReconcileCycles:    atomic.LoadUint64(&ReconcileCyclesTotal),
		ReconcileErrors:    atomic.LoadUint64(&ReconcileErrorsTotal),
		ProbeTotal:         atomic.LoadUint64(&ProbeTotal),
		ProbeFailed:        atomic.LoadUint64(&ProbeFailed),
		AssignmentSwitches: atomic.LoadUint64(&AssignmentSwitches),
		TunnelDegraded:     atomic.LoadUint64(&TunnelDegradedEvents),
		RuleSyncAdds:       atomic.LoadUint64(&RuleSyncAdds),
		RuleSyncDels:       atomic.LoadUint64(&RuleSyncDels),
		TCFlushCount:       atomic.LoadUint64(&TCFlushCount),
		TCFlushDurationMs:  atomic.LoadInt64(&TCFlushDurationMs),
	}
}

// Snapshot — копия всех счётчиков.
type Snapshot struct {
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
