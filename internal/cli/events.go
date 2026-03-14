package cli

import (
	"fmt"
	"time"

	"github.com/bslie/smartroute/internal/engine"
	"github.com/spf13/cobra"
)

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Показать последние события",
	RunE:  runEvents,
}

func runEvents(cmd *cobra.Command, args []string) error {
	// События хранятся только в памяти демона и недоступны внешнему процессу напрямую.
	// Читаем state-файл для проверки доступности демона; список событий — в будущем через IPC.
	snap, err := engine.ReadStateFile(runStateFile)
	if err != nil {
		return fmt.Errorf("демон не запущен или state-файл недоступен: %w", err)
	}
	fmt.Printf("SmartRoute запущен (generation=%d, ready=%v)\n", snap.Generation, snap.Ready)
	fmt.Printf("События доступны только в memlog демона. Используйте 'smartroute logs' для последних записей лога.\n")
	_ = time.Now()
	return nil
}
