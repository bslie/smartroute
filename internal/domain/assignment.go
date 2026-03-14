package domain

import (
	"net"
	"time"
)

// AssignmentReason — причина назначения.
type AssignmentReason string

const (
	ReasonStaticOverride   AssignmentReason = "static_override"
	ReasonComparativeScore AssignmentReason = "comparative_scoring"
	ReasonFallback        AssignmentReason = "fallback"
	ReasonStickyHold      AssignmentReason = "sticky_hold"
)

// RejectedCandidate — почему туннель не выбран.
type RejectedCandidate struct {
	TunnelName string
	Score      float64
	Reason     string
}

// Signal — сигнал для решения (latency, negative и т.д.).
type Signal struct {
	Source     string
	Confidence float64
	LatencyMs  int
	ErrorClass ErrorClass
	At         time.Time
}

// Assignment — назначение destination -> tunnel.
type Assignment struct {
	DestIP       net.IP
	TunnelName   string
	Reason       AssignmentReason
	PolicyLevel  int
	Signals      []Signal
	Score        float64
	RejectedWith []RejectedCandidate
	Generation   uint64
	CreatedAt    time.Time
	StickyCount  int
	IsSticky     bool
}
