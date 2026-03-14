package observer

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// PassiveStats — счётчики интерфейса из /proc/net/dev (только чтение).
type PassiveStats struct {
	Interface string
	RXBytes   uint64
	TXBytes   uint64
	RXErrors  uint64
	TXErrors  uint64
	At        time.Time
}

// ReadProcNetDev парсит /proc/net/dev для имени iface.
func ReadProcNetDev(iface string) (PassiveStats, error) {
	var s PassiveStats
	s.Interface = iface
	s.At = time.Now()
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return s, err
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Inter-") || strings.HasPrefix(line, "face") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		if name != iface {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) >= 4 {
			s.RXBytes, _ = strconv.ParseUint(fields[0], 10, 64)
			s.RXErrors, _ = strconv.ParseUint(fields[2], 10, 64)
		}
		if len(fields) >= 8 {
			s.TXBytes, _ = strconv.ParseUint(fields[8], 10, 64)
			s.TXErrors, _ = strconv.ParseUint(fields[10], 10, 64)
		}
		break
	}
	return s, nil
}
