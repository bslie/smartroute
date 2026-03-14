package probe

import (
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/smartroute/smartroute/internal/domain"
)

var icmpTimeRe = regexp.MustCompile(`time[=<>](\d+(?:\.\d+)?)\s*ms`)

// ICMPProbeIface выполняет ping через интерфейс (-I iface) и возвращает RTT.
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
	cmd := exec.Command("ping", args...)
	out, err := cmd.CombinedOutput()
	latencyMs := int(time.Since(start).Milliseconds())
	if err != nil {
		errClass := domain.ErrorTimeout
		if strings.Contains(string(out), "Network is unreachable") || strings.Contains(string(out), "100% packet loss") {
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
	ms := 0.0
	if m := icmpTimeRe.FindStringSubmatch(string(out)); len(m) == 2 {
		ms, _ = strconv.ParseFloat(m[1], 64)
	}
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
