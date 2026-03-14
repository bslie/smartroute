package adapter

import (
	"fmt"
	"strings"

	"github.com/bslie/smartroute/internal/domain"
)

// BuildNFTMarkRules генерирует nftables rules для установки fwmark по tunnel+class.
// table ip smartroute; chain sr_prerouting { meta mark set (meta mark & 0xffff0000) | tunnel_byte | (class_byte << 8); }
// Упрощённо: возвращает строку для nft -f.
func BuildNFTMarkRules(table string, tunnelIndexByName map[string]uint8, classIndexByClass map[domain.TrafficClass]uint8) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("table ip %s {\n", table))
	b.WriteString("  chain sr_prerouting {\n")
	// Один rule: mark = (mark & 0xffff0000) | tunnel | (class << 8) — для конкретных set'ов по dest
	// Минимальная версия: просто объявляем chain; реальные set/match добавляются при наличии decisions
	b.WriteString("    meta mark set 0x0000\n")
	b.WriteString("  }\n")
	b.WriteString("}\n")
	_ = tunnelIndexByName
	_ = classIndexByClass
	return b.String()
}

// MarkForTunnelClass возвращает fwmark для пары tunnel+class (для nft set element).
func MarkForTunnelClass(tunnelIndexOneBased uint8, class domain.TrafficClass) uint32 {
	return domain.ComposeMark(tunnelIndexOneBased, uint8(class.Index()))
}
