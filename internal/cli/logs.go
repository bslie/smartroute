package cli

import (
	"fmt"
	"time"

	"github.com/smartroute/smartroute/internal/memlog"
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
	ring := memlog.NewRing(2048)
	entries := ring.LastN(logsN)
	for _, e := range entries {
		fmt.Printf("%s [%s] %s\n", e.Time.Format(time.RFC3339), e.Level, e.Message)
	}
	return nil
}
