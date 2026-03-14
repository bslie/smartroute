package observer

import (
	"net"
	"sync"
	"time"
)

// DNSRecord — домен для IP с confidence.
type DNSRecord struct {
	Domain     string
	Confidence float64
	SeenAt     time.Time
}

// DNSCache — кэш IP -> domain (только чтение/запись кэша, без exec).
type DNSCache struct {
	mu     sync.RWMutex
	byIP   map[string][]DNSRecord
	maxAge time.Duration
}

// NewDNSCache создаёт кэш с TTL записей.
func NewDNSCache(maxAge time.Duration) *DNSCache {
	if maxAge <= 0 {
		maxAge = 300 * time.Second
	}
	return &DNSCache{byIP: make(map[string][]DNSRecord), maxAge: maxAge}
}

// Set добавляет или обновляет запись IP -> domain.
func (c *DNSCache) Set(ip net.IP, domain string, confidence float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := ip.String()
	now := time.Now()
	rec := DNSRecord{Domain: domain, Confidence: confidence, SeenAt: now}
	existing := c.byIP[key]
	// replace or append by freshness
	found := false
	for i := range existing {
		if existing[i].Domain == domain {
			existing[i] = rec
			found = true
			break
		}
	}
	if !found {
		existing = append(existing, rec)
	}
	// keep last 5 per IP
	if len(existing) > 5 {
		existing = existing[len(existing)-5:]
	}
	c.byIP[key] = existing
}

// Get возвращает лучшую запись для IP (свежая + max confidence).
func (c *DNSCache) Get(ip net.IP) (domain string, confidence float64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	key := ip.String()
	records := c.byIP[key]
	cutoff := time.Now().Add(-c.maxAge)
	var best *DNSRecord
	for i := range records {
		if records[i].SeenAt.Before(cutoff) {
			continue
		}
		if best == nil || records[i].Confidence > best.Confidence || records[i].SeenAt.After(best.SeenAt) {
			best = &records[i]
		}
	}
	if best == nil {
		return "", 0
	}
	return best.Domain, best.Confidence
}
