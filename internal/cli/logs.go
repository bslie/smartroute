package cli

import (
	"fmt"

	"github.com/bslie/smartroute/internal/engine"
	"github.com/spf13/cobra"
)

var (
	logsN int
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Последние N записей memlog",
	RunE:  runLogs,
}

func init() {
	logsCmd.Flags().IntVarP(&logsN, "lines", "n", 100, "количество строк")
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
	fmt.Printf("Журнал работы (memlog) доступен только внутри процесса демона.\n")
	return nil
}
