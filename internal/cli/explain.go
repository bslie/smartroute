package cli

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/bslie/smartroute/internal/decision"
	"github.com/bslie/smartroute/internal/domain"
	"github.com/bslie/smartroute/internal/engine"
	"github.com/bslie/smartroute/internal/store"
	"github.com/spf13/cobra"
)

var (
	explainJSON bool
)

var explainCmd = &cobra.Command{
	Use:   "explain [ip|domain|tunnel name]",
	Short: "Показать снимок решения для destination или туннеля",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runExplainOrTunnel,
}

var explainDestCmd = &cobra.Command{
	Use:   "dest [ip|domain]",
	Short: "Снимок решения для destination",
	Args:  cobra.ExactArgs(1),
	RunE:  runExplain,
}

var explainTunnelCmd = &cobra.Command{
	Use:   "tunnel [name]",
	Short: "Health и кол-во destinations по туннелю",
	Args:  cobra.ExactArgs(1),
	RunE:  runExplainTunnel,
}

func init() {
	explainCmd.AddCommand(explainDestCmd)
	explainCmd.AddCommand(explainTunnelCmd)
	explainCmd.PersistentFlags().StringVar(&explainStateFile, "state-file", "/var/run/smartroute/state.json", "файл состояния")
	explainDestCmd.Flags().BoolVar(&explainJSON, "json", false, "вывод в JSON")
}

func runExplainOrTunnel(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}
	return runExplain(cmd, args)
}

func runExplain(cmd *cobra.Command, args []string) error {
	key := args[0]
	ip := net.ParseIP(key)
	st := store.New()
	st.RLock()
	defer st.RUnlock()
	var d *domain.Destination
	if ip != nil {
		d = st.Destinations.Get(ip)
	}
	if d == nil {
		for _, dest := range st.Destinations.All() {
			if dest.Domain == key {
				d = dest
				break
			}
		}
	}
	if d == nil {
		fmt.Fprintf(os.Stderr, "destination not found: %s\n", key)
		os.Exit(1)
	}
	now := time.Now()
	profile := ""
	var snapshotAt time.Time
	if stateSnap, err := engine.ReadStateFile(explainStateFile); err == nil {
		profile = stateSnap.ActiveProfile
		snapshotAt = stateSnap.At
	}
	snap := decision.BuildSnapshot(d, now, profile, snapshotAt)
	if explainJSON {
		out, _ := decision.FormatExplainJSON(snap)
		fmt.Println(string(out))
	} else {
		fmt.Print(decision.FormatExplain(snap))
	}
	return nil
}

var explainStateFile string

func runExplainTunnel(cmd *cobra.Command, args []string) error {
	name := args[0]
	snap, err := engine.ReadStateFile(explainStateFile)
	if err != nil {
		return fmt.Errorf("state file: %w (запустите демон)", err)
	}
	// В снимке только имена; детали health — из store при следующем тике можно расширить
	found := false
	for _, n := range snap.TunnelNames {
		if n == name {
			found = true
			break
		}
	}
	if !found {
		fmt.Fprintf(os.Stderr, "tunnel not found: %s (known: %v)\n", name, snap.TunnelNames)
		os.Exit(1)
	}
	fmt.Printf("Tunnel: %s\n", name)
	fmt.Printf("  State: (from daemon; see status for generation %d)\n", snap.Generation)
	fmt.Printf("  Destinations: %d (total in system)\n", snap.DestCount)
	return nil
}
