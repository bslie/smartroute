package probe

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/bslie/smartroute/internal/domain"
)

// TCPProbe выполняет TCP connect через указанный сетевой интерфейс (iface).
// Если iface не пустой, использует SO_BINDTODEVICE для привязки сокета к интерфейсу.
// Это гарантирует, что проба идёт через конкретный туннель.
func TCPProbe(host string, port uint16, timeout time.Duration) domain.ProbeResult {
	return TCPProbeIface(host, "", port, timeout)
}

// TCPProbeIface выполняет TCP connect с привязкой к интерфейсу через SO_BINDTODEVICE.
func TCPProbeIface(host, iface string, port uint16, timeout time.Duration) domain.ProbeResult {
	start := time.Now()
	addr := net.JoinHostPort(host, "443")
	if port != 0 {
		addr = net.JoinHostPort(host, fmt.Sprintf("%d", port))
	}

	dialer := &net.Dialer{
		Timeout: timeout,
	}

	// SO_BINDTODEVICE требует root/CAP_NET_RAW
	if iface != "" {
		dialer.Control = bindToDevice(iface)
	}

	conn, err := dialer.Dial("tcp", addr)
	latencyMs := int(time.Since(start).Milliseconds())
	if err != nil {
		return domain.ProbeResult{
			DestIP:     net.ParseIP(host),
			Type:       domain.ProbeTCP,
			LatencyMs:  latencyMs,
			ErrorClass: classifyTCPErr(err),
			Confidence: 0.3,
			Timestamp:  time.Now(),
		}
	}
	conn.Close()
	conf := 0.7
	if latencyMs < 100 {
		conf = 0.9
	} else if latencyMs < 500 {
		conf = 0.7
	} else {
		conf = 0.5
	}
	return domain.ProbeResult{
		DestIP:     net.ParseIP(host),
		Type:       domain.ProbeTCP,
		LatencyMs:  latencyMs,
		ErrorClass: domain.ErrorNone,
		Confidence: conf,
		Timestamp:  time.Now(),
	}
}

func classifyTCPErr(err error) domain.ErrorClass {
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return domain.ErrorTimeout
	}
	s := err.Error()
	if strings.Contains(s, "connection refused") || strings.Contains(s, "ECONNREFUSED") {
		return domain.ErrorConnRefused
	}
	if strings.Contains(s, "no route") || strings.Contains(s, "network unreachable") || strings.Contains(s, "host unreachable") {
		return domain.ErrorTimeout
	}
	return domain.ErrorUnknown
}
