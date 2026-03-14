package cli

import (
	"fmt"
	"os"

	"github.com/bslie/smartroute/internal/domain"
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
	Use:   "add <name> <endpoint> <private_key_file>",
	Short: "Добавить туннель в конфиг",
	Args:  cobra.ExactArgs(3),
	RunE:  runTunnelAdd,
}

var tunnelRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Удалить туннель из конфига",
	Args:  cobra.ExactArgs(1),
	RunE:  runTunnelRemove,
}

func init() {
	tunnelCmd.AddCommand(tunnelListCmd)
	tunnelCmd.AddCommand(tunnelAddCmd)
	tunnelCmd.AddCommand(tunnelRemoveCmd)
	tunnelCmd.PersistentFlags().StringVarP(&tunnelConfigPath, "config", "c", "/etc/smartroute/config.yaml", "путь к конфигу")
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

func runTunnelAdd(cmd *cobra.Command, args []string) error {
	name, endpoint, keyFile := args[0], args[1], args[2]
	cfg, err := loadConfig(tunnelConfigPath)
	if err != nil {
		return err
	}
	for _, t := range cfg.Tunnels {
		if t.Name == name {
			return fmt.Errorf("tunnel %q already exists", name)
		}
	}
	cfg.Tunnels = append(cfg.Tunnels, domain.TunnelConfig{
		Name: name, Endpoint: endpoint, PrivateKeyFile: keyFile,
		RouteTable: 200 + len(cfg.Tunnels),
	})
	return saveConfig(tunnelConfigPath, cfg)
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
