package engine

import (
	"os/exec"
	"sync"
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
