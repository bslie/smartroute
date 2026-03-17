package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bslie/smartroute/internal/engine"
	"github.com/spf13/cobra"
)

var (
	logsN int
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Последние N записей memlog или лог проб",
	RunE:  runLogs,
}

var logsProbesCmd = &cobra.Command{
	Use:     "probes",
	Aliases: []string{"probs"},
	Short:   "Последние N записей лога проб (dest, tunnel, type, latency_ms, error)",
	RunE:    runLogsProbes,
}

func init() {
	logsCmd.Flags().IntVarP(&logsN, "lines", "n", 100, "количество строк")
	logsCmd.AddCommand(logsProbesCmd)
	logsProbesCmd.Flags().IntVarP(&logsN, "lines", "n", 200, "количество строк")
}

func runLogs(cmd *cobra.Command, args []string) error {
	snap, err := engine.ReadStateFile(runStateFile)
	if err != nil {
		fmt.Println("SmartRoute: демон не запущен.")
		fmt.Println("Запустите: smartroute run")
		return nil
	}
	fmt.Printf("SmartRoute: демон запущен (generation=%d, reconcile_cycles=%d, reconcile_errors=%d)\n",
		snap.Generation, snap.ReconcileCycles, snap.ReconcileErrors)
	if snap.LastReconcileError != "" {
		fmt.Println("Последняя ошибка reconcile:", snap.LastReconcileError)
	}
	fmt.Printf("Полный memlog пишется только в процесс демона. Для логов проб: smartroute logs probes\n")
	return nil
}

func runLogsProbes(cmd *cobra.Command, args []string) error {
	probeLogPath := "/var/run/smartroute/probes.log"
	if runStateFile != "" {
		probeLogPath = filepath.Join(filepath.Dir(runStateFile), "probes.log")
	}
	data, err := os.ReadFile(probeLogPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Лог проб пуст или демон ещё не записал ни одной пробы.")
			fmt.Println("Файл:", probeLogPath)
			return nil
		}
		return fmt.Errorf("чтение лога проб: %w", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		fmt.Println("Лог проб пуст.")
		return nil
	}
	n := logsN
	if n <= 0 {
		n = 200
	}
	if len(lines) < n {
		n = len(lines)
	}
	start := len(lines) - n
	fmt.Println("# time\tdest\ttunnel\ttype\tlatency_ms\terror_class\t[status_code]")
	for i := start; i < len(lines); i++ {
		fmt.Println(lines[i])
	}
	return nil
}
