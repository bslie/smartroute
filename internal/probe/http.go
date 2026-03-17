package probe

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/bslie/smartroute/internal/domain"
)

// HTTPProbeIface выполняет HTTP GET через интерфейс (SO_BINDTODEVICE) и возвращает RTT + StatusCode/ErrorClass.
// domain используется как SNI для TLS (чтобы хост выдал правильный ответ при обращении по IP).
// Сертификат не верифицируется — нас интересует доступность туннеля и HTTP-статус, а не PKI.
// Редиректы не преследуются: 3xx возвращается как ErrorNone (туннель работает).
func HTTPProbeIface(host, domainName, iface string, port uint16, timeout time.Duration) domain.ProbeResult {
	start := time.Now()
	addr := fmt.Sprintf("https://%s", host)
	if port != 0 && port != 443 {
		addr = fmt.Sprintf("https://%s:%d", host, port)
	}

	// SNI: используем доменное имя если есть, иначе host (для IP Go TLS не включает SNI в ClientHello).
	serverName := host
	if domainName != "" {
		serverName = domainName
	}

	dialer := &net.Dialer{Timeout: timeout}
	if iface != "" {
		dialer.Control = bindToDevice(iface)
	}

	client := &http.Client{
		Timeout: timeout,
		// Не следуем редиректам: 3xx = туннель работает, сервер ответил.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // проба проверяет связность туннеля, не PKI
				ServerName:         serverName,
			},
			DialContext: func(ctx context.Context, network, a string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, a)
			},
			DisableKeepAlives: true,
		},
	}

	resp, err := client.Get(addr)
	latencyMs := int(time.Since(start).Milliseconds())
	if err != nil {
		return domain.ProbeResult{
			DestIP:     net.ParseIP(host),
			Type:       domain.ProbeHTTP,
			LatencyMs:  latencyMs,
			ErrorClass: classifyHTTPErr(err),
			Confidence: 0.3,
			Timestamp:  time.Now(),
		}
	}
	defer resp.Body.Close()

	statusCode := resp.StatusCode
	errClass := classifyHTTPStatus(statusCode)
	conf := confidenceHTTP(latencyMs, statusCode)
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

func classifyHTTPErr(err error) domain.ErrorClass {
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return domain.ErrorTimeout
	}
	s := err.Error()
	if strings.Contains(s, "connection refused") || strings.Contains(s, "ECONNREFUSED") {
		return domain.ErrorConnRefused
	}
	if strings.Contains(s, "tls") || strings.Contains(s, "certificate") || strings.Contains(s, "x509") {
		return domain.ErrorTLSError
	}
	if strings.Contains(s, "no route") || strings.Contains(s, "network unreachable") {
		return domain.ErrorTimeout
	}
	return domain.ErrorUnknown
}

func classifyHTTPStatus(code int) domain.ErrorClass {
	switch {
	case code == 403:
		return domain.ErrorHTTP403
	case code == 429:
		return domain.ErrorHTTP429
	case code >= 500:
		return domain.ErrorHTTP5xx
	case code >= 400:
		return domain.ErrorHTTP4xxOther
	default:
		// 2xx, 3xx — туннель работает
		return domain.ErrorNone
	}
}

func confidenceHTTP(latencyMs, statusCode int) float64 {
	if statusCode != 200 {
		return 0.6
	}
	switch {
	case latencyMs < 100:
		return 0.9
	case latencyMs < 500:
		return 0.7
	default:
		return 0.5
	}
}
