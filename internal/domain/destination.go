package domain

import (
	"net"
	"time"
)

// DestState — состояние destination в state machine.
type DestState string

const (
	DestStateDiscovered   DestState = "discovered"
	DestStateClassified   DestState = "classified"
	DestStatePendingProbe DestState = "pending_probe"
	DestStateEvaluated    DestState = "evaluated"
	DestStateAssigned     DestState = "assigned"
	DestStateSticky       DestState = "sticky"
	DestStateStale        DestState = "stale"
	DestStateExpired      DestState = "expired"
)

// Destination — целевой хост для маршрутизации.
type Destination struct {
	IP         net.IP
	Port       uint16
	Proto      uint8
	Domain     string
	DomainConf float64 // 0.0-1.0
	Class      TrafficClass
	State      DestState
	Assignment *Assignment
	LastSeen   time.Time
	FirstSeen  time.Time
}

// DestHistory — лёгкая история после GC (для sticky при re-discovery).
type DestHistory struct {
	IP          net.IP
	Domain      string
	LastTunnel  string
	LastScore   float64
	LastSeen    time.Time
	ProbeSummary string
}
