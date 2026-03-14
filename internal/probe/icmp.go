package probe

import (
	"context"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bslie/smartroute/internal/domain"
)

// Варианты вывода ping для RTT (разные локали и версии: time=1.2 ms, time<1 ms, 1.2 ms, Round-trip 1.2/1.2 ms).
var (
	icmpTimeEqRe = regexp.MustCompile(`time[=<>](\d+(?:\.\d+)?)\s*ms`)
	icmpRttRe   = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*ms`)
)

// parsePingRTTMs извлекает RTT в миллисекундах из вывода ping (устойчиво к локали).
func parsePingRTTMs(out string) float64 {
	if m := icmpTimeEqRe.FindStringSubmatch(out); len(m) == 2 {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			return v
		}
	}
	// Fallback: первое число перед "ms" (типично RTT в строке time=... или round-trip).
	if m := icmpRttRe.FindStringSubmatch(out); len(m) == 2 {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			return v
		}
	}
	return 0
}

// ICMPProbeIface выполняет ping через интерфейс (-I iface) с таймаутом процесса и возвращает RTT.
func ICMPProbeIface(host, iface string, timeout time.Duration) domain.ProbeResult {
	start := time.Now()
	timeoutSec := int(timeout.Seconds())
	if timeoutSec < 1 {
		timeoutSec = 2
	}
	args := []string{"-c", "1", "-W", strconv.Itoa(timeoutSec)}
	if iface != "" {
		args = append(args, "-I", iface)
	}
	args = append(args, host)
	ctx, cancel := context.WithTimeout(context.Background(), timeout+time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ping", args...)
	out, err := cmd.CombinedOutput()
	latencyMs := int(time.Since(start).Milliseconds())
	if err != nil {
		errClass := domain.ErrorTimeout
		if ctx.Err() == context.DeadlineExceeded || strings.Contains(err.Error(), "deadline") {
			errClass = domain.ErrorTimeout
		} else if strings.Contains(string(out), "Network is unreachable") || strings.Contains(string(out), "100% packet loss") {
			errClass = domain.ErrorUnknown
		}
		return domain.ProbeResult{
			DestIP:     net.ParseIP(host),
			Type:       domain.ProbeICMP,
			LatencyMs:  latencyMs,
			ErrorClass: errClass,
			Confidence: 0.3,
			Timestamp:  time.Now(),
		}
	}
	ms := parsePingRTTMs(string(out))
	latencyMs = int(ms)
	if latencyMs <= 0 {
		latencyMs = int(time.Since(start).Milliseconds())
	}
	conf := 0.7
	if latencyMs < 50 {
		conf = 0.9
	} else if latencyMs < 150 {
		conf = 0.8
	}
	return domain.ProbeResult{
		DestIP:     net.ParseIP(host),
		Type:       domain.ProbeICMP,
		LatencyMs:  latencyMs,
		ErrorClass: domain.ErrorNone,
		Confidence: conf,
		Timestamp:  time.Now(),
	}
}
