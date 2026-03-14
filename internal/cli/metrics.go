package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Счётчики для мониторинга",
	RunE:  runMetrics,
}

func runMetrics(cmd *cobra.Command, args []string) error {
	fmt.Println("# SmartRoute metrics (stub)")
	fmt.Println("reconcile_cycles_total 0")
	fmt.Println("reconcile_errors_total 0")
	fmt.Println("probe_total 0")
	fmt.Println("assignment_switches_total 0")
	fmt.Println("config_generation 0")
	fmt.Println("applied_generation 0")
	return nil
}
