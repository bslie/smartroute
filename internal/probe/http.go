package probe

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"

	"github.com/smartroute/smartroute/internal/domain"
)

// HTTPProbeIface выполняет HTTP GET через интерфейс (SO_BINDTODEVICE) и возвращает RTT + StatusCode/ErrorClass.
func HTTPProbeIface(host, iface string, port uint16, timeout time.Duration) domain.ProbeResult {
	start := time.Now()
	addr := fmt.Sprintf("https://%s", host)
	if port != 0 && port != 443 {
		addr = fmt.Sprintf("https://%s:%d", host, port)
	}
	dialer := &net.Dialer{
		Timeout: timeout,
		Control: bindToDevice(iface),
	}
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, addr)
			},
		},
	}
	resp, err := client.Get(addr)
	latencyMs := int(time.Since(start).Milliseconds())
	if err != nil {
		errClass := domain.ErrorUnknown
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			errClass = domain.ErrorTimeout
		}
		return domain.ProbeResult{
			DestIP:     net.ParseIP(host),
			Type:       domain.ProbeHTTP,
			LatencyMs:  latencyMs,
			ErrorClass: errClass,
			Confidence: 0.3,
			Timestamp:  time.Now(),
		}
	}
	defer resp.Body.Close()
	statusCode := resp.StatusCode
	var errClass domain.ErrorClass
	switch {
	case statusCode == 200:
		errClass = domain.ErrorNone
	case statusCode == 403:
		errClass = domain.ErrorHTTP403
	case statusCode == 429:
		errClass = domain.ErrorHTTP429
	case statusCode >= 500:
		errClass = domain.ErrorHTTP5xx
	case statusCode >= 400:
		errClass = domain.ErrorHTTP4xxOther
	default:
		errClass = domain.ErrorNone
	}
	conf := 0.7
	if latencyMs < 100 && statusCode == 200 {
		conf = 0.9
	}
	return domain.ProbeResult{
		DestIP:     net.ParseIP(host),
		Type:       domain.ProbeHTTP,
		LatencyMs:  latencyMs,
		StatusCode: statusCode,
		ErrorClass: errClass,
		Confidence: conf,
		Timestamp:  time.Now(),
	}
}

func bindToDevice(iface string) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		var setErr error
		_ = c.Control(func(fd uintptr) {
			setErr = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, iface)
		})
		return setErr
	}
}
