package observer

import (
	"bufio"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// ConntrackEntry — одна запись из conntrack (упрощённо: dst ip:port, proto).
type ConntrackEntry struct {
	DstIP   net.IP
	DstPort uint16
	Proto   uint8
	SeenAt  time.Time
}

// ReadConntrack парсит /proc/net/nf_conntrack и возвращает уникальные dst (ip:port, proto).
// Только чтение, без мутаций.
func ReadConntrack(path string) ([]ConntrackEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	seen := make(map[string]ConntrackEntry)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		// ipv4: dst=1.2.3.4 sport=0 dport=443 ...
		if !strings.Contains(line, "dst=") {
			continue
		}
		dstIP := parseConntrackDst(line)
		if dstIP == nil {
			continue
		}
		dport := parseConntrackDport(line)
		proto := uint8(6) // TCP default
		if strings.Contains(line, "udp") {
			proto = 17
		}
		key := dstIP.String() + ":" + strconv.Itoa(int(dport)) + "/" + strconv.Itoa(int(proto))
		if _, ok := seen[key]; !ok {
			seen[key] = ConntrackEntry{DstIP: dstIP, DstPort: dport, Proto: proto, SeenAt: time.Now()}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	out := make([]ConntrackEntry, 0, len(seen))
	for _, e := range seen {
		out = append(out, e)
	}
	return out, nil
}

func parseConntrackDst(line string) net.IP {
	for _, part := range strings.Fields(line) {
		if strings.HasPrefix(part, "dst=") {
			s := strings.TrimPrefix(part, "dst=")
			ip := net.ParseIP(s)
			if ip != nil && ip.To4() != nil {
				return ip
			}
			return ip
		}
	}
	return nil
}

func parseConntrackDport(line string) uint16 {
	for _, part := range strings.Fields(line) {
		if strings.HasPrefix(part, "dport=") {
			s := strings.TrimPrefix(part, "dport=")
			n, _ := strconv.Atoi(s)
			if n >= 0 && n <= 65535 {
				return uint16(n)
			}
			return 0
		}
	}
	return 0
}
