package decision

import (
	"github.com/bslie/smartroute/internal/domain"
)

// ClassifierSource — источник классификации и его confidence.
const (
	SourceStatic   = "static"
	SourceDNSLog   = "dns_log"
	SourceSNI      = "sni"
	SourcePort     = "port_heuristic"
	SourceIPOnly   = "ip_only"
)

// ConfidenceStatic — для static mapping.
const ConfidenceStatic = 1.0

// ConfidenceDNS — для DNS log.
const ConfidenceDNS = 0.8

// ConfidenceSNI — для SNI.
const ConfidenceSNI = 0.7

// ConfidencePort — только class, не tunnel.
const ConfidencePort = 0.3

// ConfidenceIPOnly — нет class/tunnel.
const ConfidenceIPOnly = 0.1

// ClassifyResult — результат классификации destination.
type ClassifyResult struct {
	Class      domain.TrafficClass
	Confidence float64
	Source     string
	StaticTunnel string // если static override
}

// Classifier классифицирует по domain/port/static. Не импортирует os/exec.
type Classifier struct {
	StaticRoutes []domain.StaticRoute
}

// Classify возвращает class и confidence для IP/domain/port.
func (c *Classifier) Classify(ip string, domainStr string, port uint16) ClassifyResult {
	// 1) Static: domain/CIDR -> tunnel + optional class
	for _, r := range c.StaticRoutes {
		if r.Domain != "" && r.Domain == domainStr {
			class := domain.TrafficClassWeb
			if r.TrafficClass != "" {
				class = domain.TrafficClass(r.TrafficClass)
			}
			return ClassifyResult{
				Class:        class,
				Confidence:   ConfidenceStatic,
				Source:       SourceStatic,
				StaticTunnel: r.Tunnel,
			}
		}
		if r.CIDR != "" && c.matchCIDR(ip, r.CIDR) {
			class := domain.TrafficClassWeb
			if r.TrafficClass != "" {
				class = domain.TrafficClass(r.TrafficClass)
			}
			return ClassifyResult{
				Class:        class,
				Confidence:   ConfidenceStatic,
				Source:       SourceStatic,
				StaticTunnel: r.Tunnel,
			}
		}
		if r.IP != "" && r.IP == ip {
			class := domain.TrafficClassWeb
			if r.TrafficClass != "" {
				class = domain.TrafficClass(r.TrafficClass)
			}
			return ClassifyResult{
				Class:        class,
				Confidence:   ConfidenceStatic,
				Source:       SourceStatic,
				StaticTunnel: r.Tunnel,
			}
		}
	}
	// 2) Domain from DNS
	if domainStr != "" {
		return ClassifyResult{Class: domain.TrafficClassWeb, Confidence: ConfidenceDNS, Source: SourceDNSLog}
	}
	// 3) Port heuristic — class only
	if port == 443 || port == 80 {
		return ClassifyResult{Class: domain.TrafficClassWeb, Confidence: ConfidencePort, Source: SourcePort}
	}
	if port == 27015 || port == 27016 || (port >= 27000 && port <= 27200) {
		return ClassifyResult{Class: domain.TrafficClassGame, Confidence: ConfidencePort, Source: SourcePort}
	}
	// 4) IP only
	return ClassifyResult{Class: domain.TrafficClassUnknown, Confidence: ConfidenceIPOnly, Source: SourceIPOnly}
}

func (c *Classifier) matchCIDR(ipStr, cidrStr string) bool {
	// simplified: would use net.ParseIP and net.ParseCIDR + Contains
	return false
}
