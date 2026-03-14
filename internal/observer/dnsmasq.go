package observer

import (
	"net"
	"os"
	"regexp"
	"strings"
)

// Формат лога dnsmasq при log-queries: "reply example.com is 1.2.3.4" или "reply example.com is 1.2.3.4#53"
var dnsmasqReplyRe = regexp.MustCompile(`reply\s+(.+?)\s+is\s+(\S+)`)

// DnsmasqRecord — одна запись IP→domain из лога dnsmasq.
type DnsmasqRecord struct {
	IP     net.IP
	Domain string
}

// ReadDnsmasqLog читает файл лога dnsmasq (последние maxRead байт), парсит строки "reply domain is IP"
// и возвращает записи для подпитки DNSCache. При ошибке чтения возвращает nil, nil (не фатально).
func ReadDnsmasqLog(path string, maxRead int64) ([]DnsmasqRecord, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, nil
	}
	size := info.Size()
	if size > maxRead {
		_, _ = f.Seek(size-maxRead, 0)
	}
	buf := make([]byte, maxRead)
	n, _ := f.Read(buf)
	buf = buf[:n]
	lines := strings.Split(string(buf), "\n")
	var out []DnsmasqRecord
	seen := make(map[string]struct{})
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		m := dnsmasqReplyRe.FindStringSubmatch(line)
		if len(m) != 3 {
			continue
		}
		domain := strings.TrimSpace(m[1])
		ipStr := strings.TrimSpace(m[2])
		if idx := strings.Index(ipStr, "#"); idx >= 0 {
			ipStr = ipStr[:idx]
		}
		ip := net.ParseIP(ipStr)
		if ip == nil || domain == "" {
			continue
		}
		key := ip.String() + "\t" + domain
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, DnsmasqRecord{IP: ip, Domain: domain})
	}
	return out, nil
}
