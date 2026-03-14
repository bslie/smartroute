package domain

// PolicyLevel — уровень в policy stack (1–9).
const (
	PolicyLevelHardExclude     = 1
	PolicyLevelStaticOverride  = 2
	PolicyLevelGameMode        = 3
	PolicyLevelTrafficClass    = 4
	PolicyLevelNegativeExclude = 5
	PolicyLevelStickyHold      = 6
	PolicyLevelComparative     = 7
	PolicyLevelHysteresisGate  = 8
	PolicyLevelFallback        = 9
)

// RuntimeProfile — активный профиль (default/game).
type RuntimeProfile struct {
	Name              string
	RoutingPreference string // "lowest-rtt" | "balanced" | "sticky"
	HysteresisMap     map[TrafficClass]int
	StickyBonusMap    map[TrafficClass]int
	ProbeAggressive   bool
	CakeRTTMs         int
	QoSMode           string
	QoSPriorities     map[TrafficClass]int
	UDPSticky         bool
}
