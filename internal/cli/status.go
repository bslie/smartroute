package cli

import (
	"fmt"

	"github.com/smartroute/smartroute/internal/engine"
	"github.com/smartroute/smartroute/internal/store"
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
	if err == nil {
		fmt.Println("SmartRoute status (from daemon state)")
		fmt.Println("  Config generation:", snap.ConfigGeneration)
		fmt.Println("  Applied generation:", snap.AppliedConfigGen)
		fmt.Println("  Active profile:", snap.ActiveProfile)
		fmt.Println("  Tunnels:", len(snap.TunnelNames), snap.TunnelNames)
		fmt.Println("  Destinations:", snap.DestCount)
		fmt.Println("  Ready:", snap.Ready)
		if len(snap.DisabledFeat) > 0 {
			fmt.Println("  Disabled features:", snap.DisabledFeat)
		}
		fmt.Println("  State at:", snap.At)
		return nil
	}
	st := store.New()
	st.RLock()
	defer st.RUnlock()
	fmt.Println("SmartRoute status (no daemon state file)")
	fmt.Println("  Config generation: 0")
	fmt.Println("  Applied generation: 0")
	fmt.Println("  Active profile: default")
	fmt.Println("  Tunnels:", len(st.Tunnels.Names()))
	fmt.Println("  Destinations:", st.Destinations.Count())
	fmt.Println("  Ready:", st.Ready)
	return nil
}

func runStatusDestinations(cmd *cobra.Command, args []string) error {
	st := store.New()
	st.RLock()
	defer st.RUnlock()
	fmt.Println("IP\tDomain\tClass\tTunnel\tState\tAge")
	for _, d := range st.Destinations.All() {
		tunnel := "-"
		if d.Assignment != nil {
			tunnel = d.Assignment.TunnelName
		}
		fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\n", d.IP, d.Domain, d.Class, tunnel, d.State, d.LastSeen)
	}
	return nil
}
