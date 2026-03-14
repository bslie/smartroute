package engine

import (
	"os"
	"os/exec"
	"sync"

	"github.com/bslie/smartroute/internal/domain"
)

// Capabilities — доступность фич ОС (только чтение после Detect).
type Capabilities struct {
	mu sync.RWMutex

	Conntrack bool
	Nftables  bool
	TC        bool
	WireGuard bool
	DNSLog    bool
}

var defaultCaps Capabilities

func init() {
	DetectCapabilities(&defaultCaps)
}

// DetectCapabilities проверяет наличие команд/модулей.
func DetectCapabilities(c *Capabilities) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Conntrack = exec.Command("conntrack", "-V").Run() == nil
	c.Nftables = exec.Command("nft", "list", "tables").Run() == nil
	c.TC = exec.Command("tc", "-Version").Run() == nil
	c.WireGuard = exec.Command("wg", "show").Run() == nil || exec.Command("wg").Run() == nil
	// DNS log — наличие dnsmasq или файла лога
	c.DNSLog = false
}

// Get возвращает копию текущих capabilities.
func (c *Capabilities) Get() (conntrack, nft, tc, wg, dnslog bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Conntrack, c.Nftables, c.TC, c.WireGuard, c.DNSLog
}

// RefreshCapabilities заново определяет возможности ОС (вызов в bootstrap).
func RefreshCapabilities() {
	DetectCapabilities(&defaultCaps)
}

// RefreshCapabilitiesFromConfig обновляет capabilities с учётом конфига (например DNSLog при dnsmasq_log_path).
func RefreshCapabilitiesFromConfig(cfg *domain.Config) {
	if cfg == nil {
		return
	}
	defaultCaps.mu.Lock()
	defer defaultCaps.mu.Unlock()
	if cfg.DnsmasqLogPath != "" {
		if _, err := os.Stat(cfg.DnsmasqLogPath); err == nil {
			defaultCaps.DNSLog = true
		}
	}
}

// HasWireGuard возвращает true, если в системе доступна команда wg.
func HasWireGuard() bool {
	_, _, _, wg, _ := defaultCaps.Get()
	return wg
}

// DisabledFeatures возвращает список отключённых фич для status.
func (c *Capabilities) DisabledFeatures() []string {
	co, nf, tc, wg, dns := c.Get()
	var out []string
	if !co {
		out = append(out, "conntrack")
	}
	if !nf {
		out = append(out, "nftables")
	}
	if !tc {
		out = append(out, "tc")
	}
	if !wg {
		out = append(out, "wireguard")
	}
	if !dns {
		out = append(out, "dns_log")
	}
	return out
}
