package probe

import (
	"net"
	"time"

	"github.com/smartroute/smartroute/internal/domain"
)

// Result — результат пробы (alias для domain.ProbeResult).
type Result = domain.ProbeResult

// Job — задание на пробу.
type Job struct {
	DestIP   net.IP
	Tunnel   string
	Iface    string
	Type     domain.ProbeType
	Timeout  time.Duration
}
