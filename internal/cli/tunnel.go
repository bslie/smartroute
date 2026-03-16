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
	"github.com/bslie/smartroute/internal/engine"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var tunnelConfigPath string

var tunnelCmd = &cobra.Command{
	Use:   "tunnel",
	Short: "Управление туннелями (из конфига)",
}

var tunnelListCmd = &cobra.Command{
	Use:   "list",
	Short: "Список туннелей из конфига",
	RunE:  runTunnelList,
}

var tunnelAddCmd = &cobra.Command{
	Use:   "add <name> <endpoint>",
	Short: "Добавить туннель в конфиг (ключ генерируется автоматически)",
	Args:  cobra.ExactArgs(2),
	RunE:  runTunnelAdd,
}

var tunnelRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Удалить туннель из конфига",
	Args:  cobra.ExactArgs(1),
	RunE:  runTunnelRemove,
}

var tunnelSetPeerCmd = &cobra.Command{
	Use:   "set-peer <name> <public_key>",
	Short: "Задать peer_public_key (ключ сервера) для туннеля",
	Args:  cobra.ExactArgs(2),
	RunE:  runTunnelSetPeer,
}

var tunnelVPSScriptDir string
var tunnelKeysDir string

func init() {
	tunnelCmd.AddCommand(tunnelListCmd)
	tunnelCmd.AddCommand(tunnelAddCmd)
	tunnelCmd.AddCommand(tunnelRemoveCmd)
	tunnelCmd.AddCommand(tunnelSetPeerCmd)
	tunnelCmd.PersistentFlags().StringVarP(&tunnelConfigPath, "config", "c", "/etc/smartroute/config.yaml", "путь к конфигу")
	tunnelAddCmd.Flags().StringVar(&tunnelVPSScriptDir, "vps-script-dir", ".", "каталог для скрипта настройки VPS (setup-vps-<name>.sh)")
	tunnelAddCmd.Flags().StringVar(&tunnelKeysDir, "keys-dir", "", "каталог для ключей (по умолчанию: <каталог конфига>/keys)")
}

func runTunnelList(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig(tunnelConfigPath)
	if err != nil {
		return err
	}
	fmt.Println("Name\tEndpoint\tTable\tDefault")
	for _, t := range cfg.Tunnels {
		def := ""
		if t.IsDefault {
			def = "yes"
		}
		fmt.Printf("%s\t%s\t%d\t%s\n", t.Name, t.Endpoint, t.RouteTable, def)
	}
	return nil
}

func loadConfig(path string) (*domain.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	cfg := domain.DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("yaml: %w", err)
	}
	return cfg, nil
}

func saveConfig(path string, cfg *domain.Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ensureKeysDir возвращает каталог для ключей (--keys-dir или <каталог конфига>/keys), создаёт его при необходимости.
func ensureKeysDir() (string, error) {
	dir := tunnelKeysDir
	if dir == "" {
		dir = filepath.Join(filepath.Dir(tunnelConfigPath), "keys")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("создать каталог ключей %s: %w", dir, err)
	}
	return dir, nil
}

// generateKeyPair создаёт новую пару ключей для туннеля name, сохраняет приватный ключ в keysDir/name.key, возвращает путь и публичный ключ.
func generateKeyPair(keysDir, name string) (privateKeyPath, publicKey string, err error) {
	privateKeyPath = filepath.Join(keysDir, name+".key")
	cmd := exec.Command("wg", "genkey")
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("wg genkey: %w", err)
	}
	priv := bytes.TrimSpace(out)
	if err := os.WriteFile(privateKeyPath, priv, 0600); err != nil {
		return "", "", fmt.Errorf("записать ключ %s: %w", privateKeyPath, err)
	}
	pubCmd := exec.Command("wg", "pubkey")
	pubCmd.Stdin = bytes.NewReader(priv)
	pubOut, err := pubCmd.Output()
	if err != nil {
		_ = os.Remove(privateKeyPath)
		return "", "", fmt.Errorf("wg pubkey: %w", err)
	}
	return privateKeyPath, strings.TrimSpace(string(pubOut)), nil
}

func getClientPublicKey(privateKeyFile string) (string, error) {
	data, err := os.ReadFile(privateKeyFile)
	if err != nil {
		return "", fmt.Errorf("прочитать ключ: %w", err)
	}
	cmd := exec.Command("wg", "pubkey")
	cmd.Stdin = bytes.NewReader(bytes.TrimSpace(data))
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("wg pubkey: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func parseEndpointPort(endpoint string) string {
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return "51820"
	}
	_ = host
	if port == "" {
		return "51820"
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "51820"
	}
	return port
}

func generateVPSScript(name, clientPubkey, clientSubnet, listenPort string) string {
	// Примечание: конкретные значения вставляются через %s; остальные переменные bash не экранируются.
	// Heredoc в виде 'WGEOF' (кавычки) отключает интерпретацию bash внутри блока — поэтому
	// подстановка $(cat ...) внутри heredoc не работает. Вместо этого используем printf + переменную.
	hdr := fmt.Sprintf(`#!/bin/bash
# SmartRoute — настройка WireGuard на VPS (туннель: %s)
# Запускать на сервере: sudo bash setup-vps-%s.sh
# В конце скрипт выведет публичный ключ сервера — добавьте его в конфиг:
#   smartroute tunnel set-peer %s <ключ>

set -e
WG_NAME="wg0"
TUNNEL_NAME="%s"
LISTEN_PORT="%s"
CLIENT_PUBKEY="%s"
CLIENT_SUBNET="%s"
`, name, name, name, name, listenPort, clientPubkey, clientSubnet)

	body := `
if command -v wg >/dev/null 2>&1 && ip link show "$WG_NAME" >/dev/null 2>&1; then
  echo "[*] $WG_NAME уже настроен. Публичный ключ сервера:"
  wg show "$WG_NAME" public-key
  exit 0
fi

echo "[*] Установка WireGuard..."
if command -v apt-get >/dev/null 2>&1; then
  apt-get update -qq && apt-get install -y -qq wireguard wireguard-tools
elif command -v dnf >/dev/null 2>&1; then
  dnf install -y -q wireguard-tools
elif command -v yum >/dev/null 2>&1; then
  yum install -y -q wireguard-tools
elif command -v apk >/dev/null 2>&1; then
  apk add --no-cache wireguard-tools
else
  echo "Установите wireguard-tools вручную." >&2
  exit 1
fi

mkdir -p /etc/wireguard
chmod 700 /etc/wireguard

if [ ! -f "/etc/wireguard/${WG_NAME}_private.key" ]; then
  wg genkey > "/etc/wireguard/${WG_NAME}_private.key"
  wg pubkey < "/etc/wireguard/${WG_NAME}_private.key" > "/etc/wireguard/${WG_NAME}_public.key"
  chmod 600 "/etc/wireguard/${WG_NAME}_private.key"
fi

SERVER_PRIVKEY=$(cat "/etc/wireguard/${WG_NAME}_private.key")

# wg setconf не поддерживает Address — только PrivateKey, ListenPort; адрес задаём через ip ниже
umask 077
printf '[Interface]\nPrivateKey = %s\nListenPort = %s\n\n[Peer]\nPublicKey = %s\nAllowedIPs = %s\n' \
  "$SERVER_PRIVKEY" "$LISTEN_PORT" "$CLIENT_PUBKEY" "$CLIENT_SUBNET" \
  > "/etc/wireguard/${WG_NAME}.conf"
chmod 600 "/etc/wireguard/${WG_NAME}.conf"

ip link add "$WG_NAME" type wireguard 2>/dev/null || true
wg setconf "$WG_NAME" "/etc/wireguard/${WG_NAME}.conf"
ip addr add 10.0.0.1/24 dev "$WG_NAME" 2>/dev/null || true
ip link set "$WG_NAME" up

echo 1 > /proc/sys/net/ipv4/ip_forward
# NAT — пробуем основные имена интерфейсов
DEFAULT_IFACE=$(ip route show default | awk '/default/ {print $5; exit}')
if [ -n "$DEFAULT_IFACE" ]; then
  iptables -t nat -C POSTROUTING -o "$DEFAULT_IFACE" -j MASQUERADE 2>/dev/null || \
    iptables -t nat -A POSTROUTING -o "$DEFAULT_IFACE" -j MASQUERADE
fi

# Сохраняем iptables при наличии iptables-save
command -v iptables-save >/dev/null 2>&1 && iptables-save > /etc/iptables.rules 2>/dev/null || true

echo ""
echo "[OK] WireGuard на VPS настроен."
echo "Публичный ключ сервера (скопируйте в smartroute tunnel set-peer):"
wg show "$WG_NAME" public-key
echo ""
echo "Команда для клиента: smartroute tunnel set-peer $TUNNEL_NAME $(wg show "$WG_NAME" public-key)"
`
	return hdr + body
}

func runTunnelAdd(cmd *cobra.Command, args []string) error {
	name, endpoint := args[0], args[1]
	engine.RefreshCapabilities()
	if !engine.HasWireGuard() {
		if err := ensureWireGuard(); err != nil {
			return fmt.Errorf("для генерации ключей нужен WireGuard: %w", err)
		}
	}
	cfg, err := loadConfig(tunnelConfigPath)
	if err != nil {
		return err
	}
	for _, t := range cfg.Tunnels {
		if t.Name == name {
			return fmt.Errorf("tunnel %q already exists", name)
		}
	}

	keysDir, err := ensureKeysDir()
	if err != nil {
		return err
	}
	privateKeyPath, clientPubkey, err := generateKeyPair(keysDir, name)
	if err != nil {
		return fmt.Errorf("сгенерировать ключи: %w", err)
	}

	idx := len(cfg.Tunnels)
	cfg.Tunnels = append(cfg.Tunnels, domain.TunnelConfig{
		Name:           name,
		Endpoint:       endpoint,
		PrivateKeyFile: privateKeyPath,
		RouteTable:     200 + idx + 1,   // 201, 202, ...
		FWMark:         uint32(idx + 1), // 1, 2, ...
	})
	if err := saveConfig(tunnelConfigPath, cfg); err != nil {
		return err
	}

	clientSubnet := cfg.ClientSubnet
	if clientSubnet == "" {
		clientSubnet = "10.0.0.0/24"
	}
	port := parseEndpointPort(endpoint)
	scriptContent := generateVPSScript(name, clientPubkey, clientSubnet, port)
	scriptPath := filepath.Join(tunnelVPSScriptDir, "setup-vps-"+name+".sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "[!] Туннель добавлен, но не удалось записать скрипт %s: %v\n", scriptPath, err)
		return nil
	}
	fmt.Printf("Туннель %q добавлен. Ключ: %s\n", name, privateKeyPath)
	fmt.Printf("Скрипт для настройки VPS: %s\n", scriptPath)
	fmt.Printf("Скопируйте на сервер и выполните: scp %s user@vps: && ssh user@vps 'sudo bash %s'\n", scriptPath, filepath.Base(scriptPath))
	fmt.Printf("Затем добавьте ключ сервера в конфиг: smartroute tunnel set-peer %s <публичный_ключ_сервера>\n", name)
	return nil
}

func runTunnelSetPeer(cmd *cobra.Command, args []string) error {
	name, pubkey := args[0], strings.TrimSpace(args[1])
	cfg, err := loadConfig(tunnelConfigPath)
	if err != nil {
		return err
	}
	for i := range cfg.Tunnels {
		if cfg.Tunnels[i].Name == name {
			cfg.Tunnels[i].PeerPublicKey = pubkey
			if err := saveConfig(tunnelConfigPath, cfg); err != nil {
				return err
			}
			if err := applyTunnelNow(cfg.Tunnels[i], i); err != nil {
				return fmt.Errorf("ключ сохранён, но автоматическое применение туннеля не удалось: %w", err)
			}
			if waitTunnelHandshake("wg-"+name, 25*time.Second) {
				fmt.Printf("[OK] peer_public_key применён, туннель %q поднят и handshake успешен.\n", name)
				return nil
			}
			fmt.Printf("[!] peer_public_key применён для %q, но handshake пока не получен.\n", name)
			fmt.Println("Проверьте доступность endpoint, firewall/UDP-порт и запущенный WireGuard на удалённой VPS.")
			return nil
		}
	}
	return fmt.Errorf("туннель %q не найден", name)
}

func runTunnelRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
	cfg, err := loadConfig(tunnelConfigPath)
	if err != nil {
		return err
	}
	for i, t := range cfg.Tunnels {
		if t.Name == name {
			cfg.Tunnels = append(cfg.Tunnels[:i], cfg.Tunnels[i+1:]...)
			return saveConfig(tunnelConfigPath, cfg)
		}
	}
	return fmt.Errorf("tunnel %q not found", name)
}
