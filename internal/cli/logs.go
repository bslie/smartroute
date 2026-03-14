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
	// Memlog хранится только в памяти демона. Для получения логов извне нужен IPC или
	// запись в файл. Читаем state-файл как подтверждение работы демона.
	snap, err := engine.ReadStateFile(runStateFile)
	if err != nil {
		return fmt.Errorf("демон не запущен или state-файл недоступен: %w", err)
	}
	fmt.Printf("SmartRoute запущен (generation=%d, reconcile_cycles=%d, reconcile_errors=%d)\n",
		snap.Generation, snap.ReconcileCycles, snap.ReconcileErrors)
	fmt.Printf("Журнал работы (memlog) доступен только внутри процесса демона.\n")
	return nil
}
