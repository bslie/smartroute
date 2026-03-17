package domain

import (
	"net"
	"time"
)

// ProbeType — тип пробы.
type ProbeType string

const (
	ProbeTCP  ProbeType = "tcp"
	ProbeHTTP ProbeType = "http"
	ProbeICMP ProbeType = "icmp"
)

// ErrorClass — типизированный класс ошибки для negative_signal_factor.
type ErrorClass string

const (
	ErrorNone           ErrorClass = "none"
	ErrorTimeout        ErrorClass = "timeout"
	ErrorConnRefused    ErrorClass = "conn_refused"
	ErrorTLSError       ErrorClass = "tls_error"
	ErrorHTTP403        ErrorClass = "http_403"
	ErrorHTTP429        ErrorClass = "http_429"
	ErrorHTTP4xxOther   ErrorClass = "http_4xx_other"
	ErrorHTTP5xx        ErrorClass = "http_5xx"
	ErrorUnknown        ErrorClass = "unknown"
)

// ProbeResult — результат одной пробы.
type ProbeResult struct {
	DestIP     net.IP
	Domain     string // домен цели (если был в Job); пусто при dns_log отключён
	Tunnel     string
	Type       ProbeType
	LatencyMs  int
	StatusCode int
	ErrorClass ErrorClass
	Confidence float64
	Timestamp  time.Time
}

// NegativeSignalFactor возвращает множитель 0.0–1.0 для scoring.
func NegativeSignalFactor(e ErrorClass) float64 {
	switch e {
	case ErrorNone:
		return 1.0
	case ErrorHTTP403:
		return 0.1
	case ErrorHTTP429:
		return 0.5
	case ErrorHTTP4xxOther:
		return 0.3
	case ErrorHTTP5xx:
		return 0.7
	case ErrorTLSError:
		return 0.2
	case ErrorConnRefused:
		return 0.8
	case ErrorTimeout:
		return 0.4
	default:
		return 0.5
	}
}
