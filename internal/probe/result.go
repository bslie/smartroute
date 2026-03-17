package probe

import (
	"net"
	"time"

	"github.com/bslie/smartroute/internal/domain"
)

// Result — результат пробы (alias для domain.ProbeResult).
type Result = domain.ProbeResult

// Job — задание на пробу.
type Job struct {
	DestIP  net.IP
	Domain  string // доменное имя цели (для SNI в TLS); если пусто, SNI выставляется по IP
	Port    uint16 // порт назначения из conntrack; для TCP/HTTP проб
	Tunnel  string
	Iface   string
	Type    domain.ProbeType
	Timeout time.Duration
}
