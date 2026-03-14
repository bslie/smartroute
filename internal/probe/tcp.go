package probe

import (
	"fmt"
	"net"
	"time"

	"github.com/smartroute/smartroute/internal/domain"
)

// TCPProbe выполняет TCP connect и возвращает RTT и ErrorClass. Injectable.
func TCPProbe(host string, port uint16, timeout time.Duration) domain.ProbeResult {
	start := time.Now()
	addr := net.JoinHostPort(host, "443")
	if port != 0 {
		addr = net.JoinHostPort(host, fmt.Sprintf("%d", port))
	}
	// Use net.Dial for simplicity (no SO_BINDTODEVICE in stdlib; would need syscall in real impl)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	latencyMs := int(time.Since(start).Milliseconds())
	if err != nil {
		errClass := domain.ErrorUnknown
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			errClass = domain.ErrorTimeout
		}
		return domain.ProbeResult{
			DestIP:     net.ParseIP(host),
			Type:       domain.ProbeTCP,
			LatencyMs:  latencyMs,
			ErrorClass: errClass,
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
