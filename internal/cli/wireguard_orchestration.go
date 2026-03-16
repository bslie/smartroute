package cli

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bslie/smartroute/internal/domain"
)

const (
	defaultWGListenPort = 51820
)

func execCombined(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func ensureWGServer(cfg *domain.Config, cfgPath string) (string, string, bool, error) {
	if cfg == nil || cfg.WireGuardServer == nil {
		return "", "", false, nil
	}
	ws := cfg.WireGuardServer
	if ws.Interface == "" {
		return "", "", false, fmt.Errorf("wireguard_server.interface пустой")
	}
	if err := ensureWireGuard(); err != nil {
		return "", "", false, err
	}
	changed := false
	if ws.ListenPort == 0 {
		ws.ListenPort = defaultWGListenPort
		changed = true
	}
	if ws.PrivateKeyFile == "" {
		ws.PrivateKeyFile = filepath.Join(filepath.Dir(cfgPath), "keys", ws.Interface+"_server.key")
		changed = true
	}
	if ws.Address == "" && ws.PeersSubnet != "" {
		if ip, ipNet, err := net.ParseCIDR(ws.PeersSubnet); err == nil {
			base := ip.Mask(ipNet.Mask).To4()
			if base != nil {
				base[3] = 1
				maskOnes, _ := ipNet.Mask.Size()
				ws.Address = base.String() + "/" + strconv.Itoa(maskOnes)
				changed = true
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(ws.PrivateKeyFile), 0700); err != nil {
		return "", "", changed, fmt.Errorf("создать каталог ключей сервера: %w", err)
	}
	if _, err := os.Stat(ws.PrivateKeyFile); err != nil {
		out, genErr := exec.Command("wg", "genkey").Output()
		if genErr != nil {
			return "", "", changed, fmt.Errorf("wg genkey для сервера: %w", genErr)
		}
		priv := bytes.TrimSpace(out)
		if writeErr := os.WriteFile(ws.PrivateKeyFile, append(priv, '\n'), 0600); writeErr != nil {
			return "", "", changed, fmt.Errorf("запись server private key: %w", writeErr)
		}
	}
	privBytes, err := os.ReadFile(ws.PrivateKeyFile)
	if err != nil {
		return "", "", changed, fmt.Errorf("чтение server private key: %w", err)
	}
	privateKey := strings.TrimSpace(string(privBytes))
	if privateKey == "" {
		return "", "", changed, fmt.Errorf("server private key пустой: %s", ws.PrivateKeyFile)
	}

	_ = exec.Command("ip", "link", "add", "dev", ws.Interface, "type", "wireguard").Run()
	_ = exec.Command("ip", "link", "set", "up", "dev", ws.Interface).Run()

	body := buildWGServerSetconf(ws, privateKey)
	setconf := exec.Command("wg", "setconf", ws.Interface, "-")
	setconf.Stdin = strings.NewReader(body)
	if out, setErr := setconf.CombinedOutput(); setErr != nil {
		return "", "", changed, fmt.Errorf("wg setconf %s: %w: %s", ws.Interface, setErr, strings.TrimSpace(string(out)))
	}

	if ws.Address != "" {
		if err := execCombined("ip", "addr", "replace", ws.Address, "dev", ws.Interface); err != nil {
			// fallback для старых iproute2
			_ = exec.Command("ip", "addr", "add", ws.Address, "dev", ws.Interface).Run()
		}
	}
	if err := execCombined("ip", "link", "set", "up", "dev", ws.Interface); err != nil {
		return "", "", changed, err
	}
	_ = exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Run()

	natIface := strings.TrimSpace(ws.NATInterface)
	if natIface == "" {
		natIface = detectDefaultIface()
	}
	if natIface != "" {
		if err := exec.Command("iptables", "-t", "nat", "-C", "POSTROUTING", "-o", natIface, "-j", "MASQUERADE").Run(); err != nil {
			_ = exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING", "-o", natIface, "-j", "MASQUERADE").Run()
		}
	}

	pubOut, err := exec.Command("wg", "show", ws.Interface, "public-key").Output()
	if err != nil {
		return "", "", changed, fmt.Errorf("получить public key %s: %w", ws.Interface, err)
	}
	serverPub := strings.TrimSpace(string(pubOut))
	endpoint := strings.TrimSpace(ws.PublicEndpoint)
	if endpoint == "" {
		endpoint = serverEndpoint(strconv.Itoa(ws.ListenPort))
	}
	return serverPub, endpoint, changed, nil
}

func buildWGServerSetconf(ws *domain.WireGuardServerConfig, privateKey string) string {
	var b strings.Builder
	b.WriteString("[Interface]\n")
	b.WriteString("PrivateKey = ")
	b.WriteString(privateKey)
	b.WriteByte('\n')
	b.WriteString("ListenPort = ")
	b.WriteString(strconv.Itoa(ws.ListenPort))
	b.WriteByte('\n')
	for _, p := range ws.Peers {
		if strings.TrimSpace(p.PublicKey) == "" || strings.TrimSpace(p.AllowedIPs) == "" {
			continue
		}
		b.WriteString("\n[Peer]\n")
		b.WriteString("PublicKey = ")
		b.WriteString(strings.TrimSpace(p.PublicKey))
		b.WriteByte('\n')
		b.WriteString("AllowedIPs = ")
		b.WriteString(strings.TrimSpace(p.AllowedIPs))
		b.WriteByte('\n')
	}
	return b.String()
}

func detectDefaultIface() string {
	out, err := exec.Command("sh", "-c", "ip route show default | awk '/default/ {print $5; exit}'").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func applyTunnelNow(t domain.TunnelConfig, index int) error {
	if err := ensureWireGuard(); err != nil {
		return err
	}
	if t.PrivateKeyFile == "" {
		return fmt.Errorf("tunnel %s: private_key_file не задан", t.Name)
	}
	if strings.TrimSpace(t.PeerPublicKey) == "" {
		return fmt.Errorf("tunnel %s: peer_public_key пустой", t.Name)
	}
	if t.Endpoint == "" {
		return fmt.Errorf("tunnel %s: endpoint пустой", t.Name)
	}

	iface := "wg-" + t.Name
	_ = exec.Command("ip", "link", "add", "dev", iface, "type", "wireguard").Run()
	_ = exec.Command("ip", "link", "set", "up", "dev", iface).Run()

	priv, err := os.ReadFile(t.PrivateKeyFile)
	if err != nil {
		return fmt.Errorf("прочитать private key %s: %w", t.PrivateKeyFile, err)
	}
	privKey := strings.TrimSpace(string(priv))
	if privKey == "" {
		return fmt.Errorf("private key пустой: %s", t.PrivateKeyFile)
	}

	var b strings.Builder
	b.WriteString("[Interface]\nPrivateKey = ")
	b.WriteString(privKey)
	b.WriteString("\n\n[Peer]\nPublicKey = ")
	b.WriteString(strings.TrimSpace(t.PeerPublicKey))
	b.WriteString("\nEndpoint = ")
	b.WriteString(strings.TrimSpace(t.Endpoint))
	b.WriteString("\nAllowedIPs = 0.0.0.0/0, ::/0\nPersistentKeepalive = 25\n")
	setconf := exec.Command("wg", "setconf", iface, "-")
	setconf.Stdin = strings.NewReader(b.String())
	if out, setErr := setconf.CombinedOutput(); setErr != nil {
		return fmt.Errorf("wg setconf %s: %w: %s", iface, setErr, strings.TrimSpace(string(out)))
	}
	if err := execCombined("ip", "link", "set", "up", "dev", iface); err != nil {
		return err
	}
	rt := t.RouteTable
	if rt == 0 {
		rt = 200 + index + 1
	}
	_ = execCombined("ip", "route", "replace", "default", "dev", iface, "table", strconv.Itoa(rt))
	return nil
}

func waitTunnelHandshake(iface string, timeout time.Duration) bool {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := exec.Command("wg", "show", iface, "latest-handshakes").Output()
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			for _, ln := range lines {
				parts := strings.Fields(strings.TrimSpace(ln))
				if len(parts) >= 2 {
					if ts, parseErr := strconv.ParseInt(parts[1], 10, 64); parseErr == nil && ts > 0 {
						return true
					}
				}
			}
		}
		_ = exec.Command("ping", "-I", iface, "-c", "1", "-W", "1", "1.1.1.1").Run()
		time.Sleep(2 * time.Second)
	}
	return false
}
