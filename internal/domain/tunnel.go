package domain

import "time"

// TunnelState — состояние туннеля в state machine.
type TunnelState string

const (
	TunnelStateDeclared     TunnelState = "declared"
	TunnelStateProvisioning TunnelState = "provisioning"
	TunnelStateActive       TunnelState = "active"
	TunnelStateDegraded     TunnelState = "degraded"
	TunnelStateQuarantined  TunnelState = "quarantined"
	TunnelStateUnavailable TunnelState = "unavailable"
	TunnelStateFailed       TunnelState = "failed"
	TunnelStateDisabled     TunnelState = "disabled"
)

// LivenessStatus — живой ли интерфейс/handshake.
type LivenessStatus string

const (
	LivenessUp      LivenessStatus = "up"
	LivenessDown    LivenessStatus = "down"
	LivenessUnknown LivenessStatus = "unknown"
)

// PerformanceStatus — качество канала.
type PerformanceStatus string

const (
	PerformanceGood    PerformanceStatus = "good"
	PerformanceDegraded PerformanceStatus = "degraded"
	PerformancePoor    PerformanceStatus = "poor"
)

// TunnelHealth — здоровье туннеля.
type TunnelHealth struct {
	Liveness        LivenessStatus
	Performance     PerformanceStatus
	Score           float64 // 0.0 (dead) - 1.0 (perfect)
	PenaltyMs       int
	LastCheck       time.Time
	DegradedAt      time.Time
	HandshakeAgeSec int     // секунды с последнего WG handshake; -1 = неизвестно
	PacketLossPct   float64 // оценка потери пакетов (0–100)
}

// Tunnel — туннель WireGuard.
type Tunnel struct {
	Name       string
	Endpoint   string
	Interface  string
	RouteTable int
	FWMark     uint32
	IsDefault  bool
	State      TunnelState
	Health     TunnelHealth
	Disabled   bool
}
