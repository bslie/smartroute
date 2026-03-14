package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	gameModeFile string
)

var gameCmd = &cobra.Command{
	Use:   "game",
	Short: "Game mode: переключение профиля (on/off)",
}

var gameOnCmd = &cobra.Command{
	Use:   "on",
	Short: "Включить game profile",
	RunE:  runGameSet,
}

var gameOffCmd = &cobra.Command{
	Use:   "off",
	Short: "Выключить (default profile)",
	RunE:  runGameSet,
}

func init() {
	gameCmd.AddCommand(gameOnCmd)
	gameCmd.AddCommand(gameOffCmd)
	gameCmd.PersistentFlags().StringVar(&gameModeFile, "mode-file", "/var/run/smartroute/game_mode", "файл для передачи профиля демону")
}

func runGameSet(cmd *cobra.Command, args []string) error {
	profile := "default"
	if cmd.Name() == "on" {
		profile = "game"
	}
	dir := gameModeFile
	for i := len(dir) - 1; i >= 0; i-- {
		if dir[i] == '/' {
			dir = dir[:i]
			break
		}
	}
	if dir != gameModeFile {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("mkdir: %w", err)
		}
	}
	if err := os.WriteFile(gameModeFile, []byte(strings.TrimSpace(profile)), 0644); err != nil {
		return fmt.Errorf("write mode file: %w", err)
	}
	fmt.Println("Profile set to:", profile)
	return nil
}
