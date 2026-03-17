package cli

import (
	"fmt"

	"github.com/bslie/smartroute/internal/engine"
	"github.com/spf13/cobra"
)

var metricsStateFile string

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Счётчики для мониторинга",
	RunE:  runMetrics,
}

func init() {
	metricsCmd.Flags().StringVar(&metricsStateFile, "state-file", "/var/run/smartroute/state.json", "файл состояния демона")
}

func runMetrics(cmd *cobra.Command, args []string) error {
	snap, err := engine.ReadStateFile(metricsStateFile)
	if err != nil {
		// Демон не запущен или state file недоступен — выводим нули
		fmt.Println("# SmartRoute metrics (daemon not running or state-file not found)")
		fmt.Println("dest_count 0")
		fmt.Println("reconcile_cycles_total 0")
		fmt.Println("reconcile_errors_total 0")
		fmt.Println("probe_total 0")
		fmt.Println("probe_failed_total 0")
		fmt.Println("assignment_switches_total 0")
		fmt.Println("tunnel_degraded_events_total 0")
		fmt.Println("rule_sync_adds 0")
		fmt.Println("rule_sync_dels 0")
		fmt.Println("tc_flush_count 0")
		fmt.Println("tc_flush_duration_ms 0")
		fmt.Println("config_generation 0")
		fmt.Println("applied_generation 0")
		return nil
	}
	fmt.Println("# SmartRoute metrics")
	fmt.Printf("dest_count %d\n", snap.DestCount)
	if snap.ProbeTotal == 0 {
		fmt.Println("# probe_total=0: пробы выполняются при Destinations>0 (трафик в conntrack). См. dest_count и smartroute status.")
	}
	fmt.Printf("reconcile_cycles_total %d\n", snap.ReconcileCycles)
	fmt.Printf("reconcile_errors_total %d\n", snap.ReconcileErrors)
	fmt.Printf("probe_total %d\n", snap.ProbeTotal)
	fmt.Printf("probe_failed_total %d\n", snap.ProbeFailed)
	fmt.Printf("assignment_switches_total %d\n", snap.AssignmentSwitches)
	fmt.Printf("tunnel_degraded_events_total %d\n", snap.TunnelDegraded)
	fmt.Printf("rule_sync_adds %d\n", snap.RuleSyncAdds)
	fmt.Printf("rule_sync_dels %d\n", snap.RuleSyncDels)
	fmt.Printf("tc_flush_count %d\n", snap.TCFlushCount)
	fmt.Printf("tc_flush_duration_ms %d\n", snap.TCFlushDurationMs)
	fmt.Printf("config_generation %d\n", snap.ConfigGeneration)
	fmt.Printf("applied_generation %d\n", snap.AppliedConfigGen)
	return nil
}
