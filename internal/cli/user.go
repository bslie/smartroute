package cli

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/smartroute/smartroute/internal/domain"
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
	Short: "Добавить пользователя (peer). Если allowed_ips не указан, выдаётся следующий IP из peers_subnet.",
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

	// Генерируем ключи
	privateKey, err := exec.Command("wg", "genkey").Output()
	if err != nil {
		return fmt.Errorf("wg genkey: %w", err)
	}
	pubCmd := exec.Command("wg", "pubkey")
	pubCmd.Stdin = strings.NewReader(strings.TrimSpace(string(privateKey)))
	publicKey, err := pubCmd.Output()
	if err != nil {
		return fmt.Errorf("wg pubkey: %w", err)
	}
	pubStr := strings.TrimSpace(string(publicKey))

	peer := domain.PeerConfig{Name: name, PublicKey: pubStr, AllowedIPs: allowedIPs}
	ws.Peers = append(ws.Peers, peer)

	if err := saveConfig(userConfigPath, cfg); err != nil {
		return err
	}

	// Применяем на интерфейс
	if err := wgSetPeer(ws.Interface, pubStr, allowedIPs); err != nil {
		return fmt.Errorf("добавлен в конфиг, но wg set не выполнен: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[OK] Пользователь %q добавлен. AllowedIPs: %s\n", name, allowedIPs)
	fmt.Println(strings.TrimSpace(string(privateKey)))
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

	if err := wgRemovePeer(ws.Interface, pubKey); err != nil {
		return fmt.Errorf("удалён из конфига, но wg set remove не выполнен: %w", err)
	}
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

	if err := wgSetPeer(ws.Interface, pubKey, userEditAllowedIPs); err != nil {
		return fmt.Errorf("конфиг обновлён, но wg set не выполнен: %w", err)
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
	// Ищем следующий свободный /32 в подсети (начинаем с .2, .3, ...)
	ones, bits := subnet.Mask.Size()
	if bits != 32 || ones >= 31 {
		return "", fmt.Errorf("peers_subnet должен быть IPv4 подсеть (например 10.0.0.0/24)")
	}
	ip := subnet.IP.To4()
	if ip == nil {
		return "", fmt.Errorf("только IPv4")
	}
	// Пропускаем .0 и .1
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

