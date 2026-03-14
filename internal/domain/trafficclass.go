package domain

// TrafficClass — класс трафика для policy/QoS.
type TrafficClass string

const (
	TrafficClassGame   TrafficClass = "game"
	TrafficClassWeb    TrafficClass = "web"
	TrafficClassStream TrafficClass = "stream"
	TrafficClassBulk   TrafficClass = "bulk"
	TrafficClassUnknown TrafficClass = "unknown"
)

// ClassIndex для fwmark bits [8:15].
func (t TrafficClass) Index() int {
	switch t {
	case TrafficClassGame:
		return 1
	case TrafficClassWeb:
		return 2
	case TrafficClassStream:
		return 3
	case TrafficClassBulk:
		return 4
	default:
		return 0
	}
}
