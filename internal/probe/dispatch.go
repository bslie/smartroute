package probe

import (
	"time"

	"github.com/bslie/smartroute/internal/domain"
)

// RunProbe выполняет пробу по типу задания (TCP, HTTP, ICMP).
func RunProbe(j Job) domain.ProbeResult {
	timeout := j.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	host := ""
	if j.DestIP != nil {
		host = j.DestIP.String()
	}
	if host == "" {
		return domain.ProbeResult{ErrorClass: domain.ErrorUnknown, Timestamp: time.Now()}
	}
	switch j.Type {
	case domain.ProbeHTTP:
		return HTTPProbeIface(host, j.Iface, 443, timeout)
	case domain.ProbeICMP:
		return ICMPProbeIface(host, j.Iface, timeout)
	default:
		return TCPProbeIface(host, j.Iface, 443, timeout)
	}
}
