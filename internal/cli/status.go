package cli

import (
	"fmt"

	"github.com/bslie/smartroute/internal/engine"
	"github.com/spf13/cobra"
)

var statusStateFile string

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Показать статус системы",
}

var statusDestinationsCmd = &cobra.Command{
	Use:   "destinations",
	Short: "Таблица destinations",
	RunE:  runStatusDestinations,
}

func init() {
	statusCmd.AddCommand(statusDestinationsCmd)
	statusCmd.RunE = runStatus
	statusCmd.Flags().StringVar(&statusStateFile, "state-file", "/var/run/smartroute/state.json", "файл состояния (как у демона)")
}

func runStatus(cmd *cobra.Command, args []string) error {
	snap, err := engine.ReadStateFile(statusStateFile)
	if err != nil {
		fmt.Println("SmartRoute: демон не запущен")
		fmt.Println("  Ready: false")
		fmt.Println("Запустите: smartroute run")
		return nil
	}
	fmt.Println("SmartRoute: демон запущен")
	fmt.Println("  Ready:            ", snap.Ready)
	fmt.Println("  Active profile:   ", snap.ActiveProfile)
	fmt.Println("  Config generation:", snap.ConfigGeneration)
	fmt.Println("  Applied:          ", snap.AppliedConfigGen)
	fmt.Println("  Tunnels:          ", snap.TunnelNames)
	fmt.Println("  Destinations:     ", snap.DestCount)
	fmt.Printf("  Conntrack (тик):   %d записей → %d уникальных dst\n", snap.ConntrackEntries, snap.DestCount)
	fmt.Println("  Пробы (задержки): при Destinations > 0; счётчики — smartroute metrics (probe_total, probe_failed_total)")
	if len(snap.DisabledFeat) > 0 {
		fmt.Println("  Disabled features:", snap.DisabledFeat)
	}
	if snap.LastReconcileError != "" {
		fmt.Println("  Last reconcile err:", snap.LastReconcileError)
	}
	fmt.Println("  State at:         ", snap.At)
	return nil
}

func runStatusDestinations(cmd *cobra.Command, args []string) error {
	snap, err := engine.ReadStateFile(statusStateFile)
	if err != nil {
		fmt.Println("SmartRoute: демон не запущен, destinations недоступны.")
		return nil
	}
	fmt.Printf("SmartRoute: destinations (generation=%d, count=%d)\n", snap.Generation, snap.DestCount)
	fmt.Println("Для детального просмотра destinations запустите демон и используйте smartroute dump")
	return nil
}
