package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/bslie/smartroute/internal/decision"
	"github.com/bslie/smartroute/internal/engine"
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
	key := args[0]
	// Если аргумент — имя туннеля из state-файла, показываем снимок туннеля.
	if snap, err := engine.ReadStateFile(explainStateFile); err == nil {
		for _, n := range snap.TunnelNames {
			if n == key {
				return runExplainTunnel(cmd, args)
			}
		}
	}
	return runExplain(cmd, args)
}

func runExplain(cmd *cobra.Command, args []string) error {
	key := args[0]
	stateSnap, err := engine.ReadStateFile(explainStateFile)
	if err != nil {
		return fmt.Errorf("state file: %w (запустите демон: smartroute run)", err)
	}
	rec := engine.FindDestinationRecord(stateSnap, key)
	if rec == nil {
		msg := fmt.Sprintf("destination not found: %s (для туннеля: smartroute explain <tunnel_name>)", key)
		if stateSnap.DestinationsTruncated {
			msg += "\n  Примечание: список destinations в state усечён (destinations_truncated); цель может быть среди не попавших записей."
		}
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
	d, err := engine.DomainDestinationFromRecord(rec)
	if err != nil || d == nil {
		return fmt.Errorf("разбор destination из state: %w", err)
	}
	now := time.Now()
	profile := stateSnap.ActiveProfile
	snapshotAt := stateSnap.At
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
	assigned := 0
	if snap.DestCountByTunnel != nil {
		assigned = snap.DestCountByTunnel[name]
	}
	fmt.Printf("  Assigned: %d destinations (of %d total)\n", assigned, snap.DestCount)
	fmt.Printf("  Generation: %d\n", snap.Generation)
	if contains(snap.DisabledFeat, "dns_log") {
		fmt.Println("  Примечание: домены недоступны (dns_log отключён) — только TCP/ICMP пробы, HTTP и детекция geo-block (403) не работают.")
	}
	return nil
}

func contains(s []string, x string) bool {
	for _, v := range s {
		if v == x {
			return true
		}
	}
	return false
}
