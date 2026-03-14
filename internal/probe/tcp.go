package probe

import (
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/smartroute/smartroute/internal/domain"
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

// bindToDevice возвращает Control func для net.Dialer, устанавливающую SO_BINDTODEVICE.
func bindToDevice(iface string) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		var setSockOptErr error
		err := c.Control(func(fd uintptr) {
			setSockOptErr = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, iface)
		})
		if err != nil {
			return err
		}
		return setSockOptErr
	}
}
