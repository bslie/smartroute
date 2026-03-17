package cli

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bslie/smartroute/internal/domain"
	"github.com/spf13/cobra"
)

var userConfigPath string

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Управление пользователями (peer'ами) WireGuard-сервера",
	Long:  "Добавление, удаление и редактирование peer'ов на интерфейсе WireGuard. Требуется секция wireguard_server в конфиге.",
}

var userListCmd = &cobra.Command{
	Use:   "list",
	Short: "Список пользователей (peer'ов) из конфига",
	RunE:  runUserList,
}

var userAddCmd = &cobra.Command{
	Use:   "add <name> [allowed_ips]",
	Short: "Добавить пользователя: генерирует ключи, сохраняет конфиг, показывает QR-код",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runUserAdd,
}

var userRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Удалить пользователя (peer)",
	Args:  cobra.ExactArgs(1),
	RunE:  runUserRemove,
}

var userEditCmd = &cobra.Command{
	Use:   "edit <name>",
	Short: "Изменить allowed_ips пользователя",
	Args:  cobra.ExactArgs(1),
	RunE:  runUserEdit,
}

var userEditAllowedIPs string

func init() {
	userCmd.AddCommand(userListCmd)
	userCmd.AddCommand(userAddCmd)
	userCmd.AddCommand(userRemoveCmd)
	userCmd.AddCommand(userEditCmd)
	userCmd.PersistentFlags().StringVarP(&userConfigPath, "config", "c", "/etc/smartroute/config.yaml", "путь к конфигу")
	userEditCmd.Flags().StringVar(&userEditAllowedIPs, "allowed-ips", "", "новые allowed_ips (например 10.0.0.5/32)")
	_ = userEditCmd.MarkFlagRequired("allowed-ips")
}

func requireWireGuardServer(cfg *domain.Config) (*domain.WireGuardServerConfig, error) {
	if cfg.WireGuardServer == nil || cfg.WireGuardServer.Interface == "" {
		return nil, fmt.Errorf("в конфиге должна быть секция wireguard_server с полем interface (например wg0)")
	}
	return cfg.WireGuardServer, nil
}

// ensureDefaultWireGuardServer добавляет секцию wireguard_server с дефолтами, если её нет. Возвращает true, если конфиг изменён.
func ensureDefaultWireGuardServer(cfg *domain.Config, configPath string) bool {
	if cfg.WireGuardServer != nil && cfg.WireGuardServer.Interface != "" {
		return false
	}
	cfgDir := filepath.Dir(configPath)
	cfg.WireGuardServer = &domain.WireGuardServerConfig{
		Interface:      "wg0",
		Address:        "10.100.0.1/24",
		ListenPort:     51820,
		PrivateKeyFile: filepath.Join(cfgDir, "keys", "wg0_server.key"),
		PeersSubnet:    "10.100.0.0/24",
		Peers:          nil,
	}
	return true
}

func runUserList(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig(userConfigPath)
	if err != nil {
		return err
	}
	ws, err := requireWireGuardServer(cfg)
	if err != nil {
		return err
	}
	if len(ws.Peers) == 0 {
		fmt.Println("Пользователей нет.")
		return nil
	}
	fmt.Println("Name\tAllowedIPs\tPublicKey")
	for _, p := range ws.Peers {
		pubShort := p.PublicKey
		if len(pubShort) > 20 {
			pubShort = pubShort[:20] + "..."
		}
		fmt.Printf("%s\t%s\t%s\n", p.Name, p.AllowedIPs, pubShort)
	}
	return nil
}

// serverEndpoint пытается определить внешний IP сервера (для показа в клиентском конфиге).
func serverEndpoint(listenPort string) string {
	// Пробуем получить внешний IP через ip route и curl; если не получилось — подсказка-заглушка
	out, err := exec.Command("sh", "-c", `curl -s --max-time 3 ifconfig.me || curl -s --max-time 3 api.ipify.org || true`).Output()
	ip := strings.TrimSpace(string(out))
	if err != nil || ip == "" || net.ParseIP(ip) == nil {
		return "<IP_СЕРВЕРА>:" + listenPort
	}
	return ip + ":" + listenPort
}

// userKeysDir возвращает каталог для ключей пользователей: <каталог конфига>/peers.
func userKeysDir() string {
	return filepath.Join(filepath.Dir(userConfigPath), "peers")
}

// generateUserKeyPair генерирует пару ключей для пользователя, сохраняет приватный ключ.
func generateUserKeyPair(name string) (privKeyPath, privKey, pubKey string, err error) {
	dir := userKeysDir()
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", "", "", fmt.Errorf("создать каталог %s: %w", dir, err)
	}
	privKeyPath = filepath.Join(dir, name+".key")

	privOut, err := exec.Command("wg", "genkey").Output()
	if err != nil {
		return "", "", "", fmt.Errorf("wg genkey: %w", err)
	}
	privKey = strings.TrimSpace(string(privOut))
	if err = os.WriteFile(privKeyPath, []byte(privKey+"\n"), 0600); err != nil {
		return "", "", "", fmt.Errorf("записать ключ %s: %w", privKeyPath, err)
	}

	pubCmd := exec.Command("wg", "pubkey")
	pubCmd.Stdin = strings.NewReader(privKey)
	pubOut, err := pubCmd.Output()
	if err != nil {
		_ = os.Remove(privKeyPath)
		return "", "", "", fmt.Errorf("wg pubkey: %w", err)
	}
	pubKey = strings.TrimSpace(string(pubOut))
	return privKeyPath, privKey, pubKey, nil
}

// buildClientConfig формирует текст WG-конфига для клиентского устройства.
func buildClientConfig(privKey, allowedIPs, serverPubKey, serverEndpoint string) string {
	// IP интерфейса клиента = allowedIPs (например 10.0.0.2/32)
	return fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s
DNS = 1.1.1.1

[Peer]
PublicKey = %s
Endpoint = %s
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = 25
`, privKey, allowedIPs, serverPubKey, serverEndpoint)
}

// printQR выводит QR-код конфига в терминал через qrencode (если доступен).
func printQR(content string) {
	cmd := exec.Command("qrencode", "-t", "UTF8", "-l", "L")
	cmd.Stdin = bytes.NewBufferString(content)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("[!] QR-код не показан: установите qrencode (apt install qrencode)")
	}
}

func runUserAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	var allowedIPs string
	if len(args) >= 2 {
		allowedIPs = args[1]
	}

	cfg, err := loadConfig(userConfigPath)
	if err != nil {
		return err
	}
	if ensureDefaultWireGuardServer(cfg, userConfigPath) {
		if err := saveConfig(userConfigPath, cfg); err != nil {
			return fmt.Errorf("создать секцию wireguard_server в конфиге: %w", err)
		}
		fmt.Fprintf(os.Stderr, "[*] В конфиг добавлена секция wireguard_server (wg0, 10.100.0.0/24).\n")
	}
	ws, err := requireWireGuardServer(cfg)
	if err != nil {
		return err
	}
	for _, p := range ws.Peers {
		if p.Name == name {
			return fmt.Errorf("пользователь %q уже существует", name)
		}
	}

	if allowedIPs == "" {
		allowedIPs, err = nextAllowedIP(ws)
		if err != nil {
			return fmt.Errorf("не заданы allowed_ips и не удалось выдать из peers_subnet: %w", err)
		}
	}
	serverPub, serverEP, changed, err := ensureWGServer(cfg, userConfigPath)
	if err != nil {
		return fmt.Errorf("не удалось подготовить wireguard_server: %w", err)
	}
	if changed {
		if err := saveConfig(userConfigPath, cfg); err != nil {
			return err
		}
	}

	privKeyPath, privKey, pubKey, err := generateUserKeyPair(name)
	if err != nil {
		return err
	}

	peer := domain.PeerConfig{Name: name, PublicKey: pubKey, AllowedIPs: allowedIPs}
	ws.Peers = append(ws.Peers, peer)

	if err := saveConfig(userConfigPath, cfg); err != nil {
		return err
	}
	serverPub, serverEP, changed, err = ensureWGServer(cfg, userConfigPath)
	if changed {
		if saveErr := saveConfig(userConfigPath, cfg); saveErr != nil {
			return saveErr
		}
	}
	if err != nil {
		return fmt.Errorf("конфиг сохранён, но не удалось применить peer на сервере: %w", err)
	}

	clientCfg := buildClientConfig(privKey, allowedIPs, serverPub, serverEP)

	// Сохраняем клиентский конфиг рядом с ключом
	cfgPath := filepath.Join(userKeysDir(), name+".conf")
	_ = os.WriteFile(cfgPath, []byte(clientCfg), 0600)

	fmt.Printf("\n[OK] Пользователь %q добавлен. AllowedIPs: %s\n", name, allowedIPs)
	fmt.Printf("Ключ сохранён: %s\n", privKeyPath)
	fmt.Printf("Конфиг клиента: %s\n\n", cfgPath)
	fmt.Println("=== Конфиг для WireGuard-приложения (скопируйте вручную или отсканируйте QR) ===")
	fmt.Println(clientCfg)
	fmt.Println("=== QR-код ===")
	printQR(clientCfg)
	return nil
}

func runUserRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
	cfg, err := loadConfig(userConfigPath)
	if err != nil {
		return err
	}
	ws, err := requireWireGuardServer(cfg)
	if err != nil {
		return err
	}

	var pubKey string
	var newPeers []domain.PeerConfig
	for _, p := range ws.Peers {
		if p.Name == name {
			pubKey = p.PublicKey
			continue
		}
		newPeers = append(newPeers, p)
	}
	if pubKey == "" {
		return fmt.Errorf("пользователь %q не найден", name)
	}
	ws.Peers = newPeers

	if err := saveConfig(userConfigPath, cfg); err != nil {
		return err
	}
	if _, _, changed, err := ensureWGServer(cfg, userConfigPath); err != nil {
		return fmt.Errorf("пользователь удалён из конфига, но применить wireguard_server не удалось: %w", err)
	} else if changed {
		if err := saveConfig(userConfigPath, cfg); err != nil {
			return err
		}
	}

	// Удаляем файлы ключа и конфига
	dir := userKeysDir()
	_ = os.Remove(filepath.Join(dir, name+".key"))
	_ = os.Remove(filepath.Join(dir, name+".conf"))

	fmt.Printf("[OK] Пользователь %q удалён.\n", name)
	return nil
}

func runUserEdit(cmd *cobra.Command, args []string) error {
	name := args[0]
	cfg, err := loadConfig(userConfigPath)
	if err != nil {
		return err
	}
	ws, err := requireWireGuardServer(cfg)
	if err != nil {
		return err
	}

	var pubKey string
	for i := range ws.Peers {
		if ws.Peers[i].Name == name {
			pubKey = ws.Peers[i].PublicKey
			ws.Peers[i].AllowedIPs = userEditAllowedIPs
			break
		}
	}
	if pubKey == "" {
		return fmt.Errorf("пользователь %q не найден", name)
	}

	if err := saveConfig(userConfigPath, cfg); err != nil {
		return err
	}
	if _, _, changed, err := ensureWGServer(cfg, userConfigPath); err != nil {
		return fmt.Errorf("конфиг обновлён, но применить wireguard_server не удалось: %w", err)
	} else if changed {
		if err := saveConfig(userConfigPath, cfg); err != nil {
			return err
		}
	}
	fmt.Printf("[OK] Пользователь %q обновлён: allowed_ips=%s\n", name, userEditAllowedIPs)
	return nil
}

func nextAllowedIP(ws *domain.WireGuardServerConfig) (string, error) {
	if ws.PeersSubnet == "" {
		return "", fmt.Errorf("задайте wireguard_server.peers_subnet в конфиге или укажите allowed_ips вручную")
	}
	_, subnet, err := net.ParseCIDR(ws.PeersSubnet)
	if err != nil {
		return "", err
	}
	used := make(map[string]bool)
	for _, p := range ws.Peers {
		ip, _, err := net.ParseCIDR(p.AllowedIPs)
		if err != nil {
			continue
		}
		used[ip.String()] = true
	}
	ones, bits := subnet.Mask.Size()
	if bits != 32 || ones >= 31 {
		return "", fmt.Errorf("peers_subnet должен быть IPv4 подсеть (например 10.0.0.0/24)")
	}
	ip := subnet.IP.To4()
	if ip == nil {
		return "", fmt.Errorf("только IPv4")
	}
	// Пропускаем .0 и .1 (сеть и шлюз)
	for i := 2; i < 256; i++ {
		ip[3] = byte(i)
		if !subnet.Contains(ip) {
			break
		}
		candidate := ip.String() + "/32"
		if !used[ip.String()] {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("нет свободных IP в подсети %s", ws.PeersSubnet)
}

func wgSetPeer(iface, publicKey, allowedIPs string) error {
	cmd := exec.Command("wg", "set", iface, "peer", strings.TrimSpace(publicKey), "allowed-ips", allowedIPs)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, string(out))
	}
	return nil
}

func wgRemovePeer(iface, publicKey string) error {
	cmd := exec.Command("wg", "set", iface, "peer", strings.TrimSpace(publicKey), "remove")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, string(out))
	}
	return nil
}
