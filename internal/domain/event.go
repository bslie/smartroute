package domain

import (
	"net"
	"time"
)

// EventType — тип события.
type EventType string

const (
	EventSystemReady        EventType = "system_ready"
	EventSystemShutdown      EventType = "system_shutdown"
	EventTunnelDegraded     EventType = "tunnel_degraded"
	EventTunnelRecovered    EventType = "tunnel_recovered"
	EventTunnelQuarantined  EventType = "tunnel_quarantined"
	EventTunnelUnavailable  EventType = "tunnel_unavailable"
	EventAllTunnelsDown     EventType = "all_tunnels_down"
	EventAssignmentSwitched EventType = "assignment_switched"
	EventConfigReloaded     EventType = "config_reloaded"
	EventConfigRejected     EventType = "config_rejected"
	EventReconcileFailed    EventType = "reconcile_failed"
	EventFeatureDisabled   EventType = "feature_disabled"
)

// Severity — уровень важности.
type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Event — событие для observability.
type Event struct {
	Type      EventType
	Timestamp time.Time
	Severity  Severity
	Tunnel    string
	DestIP    net.IP
	Message   string
	Data      map[string]string
}
