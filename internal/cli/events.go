package cli

import (
	"fmt"

	"github.com/bslie/smartroute/internal/engine"
	"github.com/spf13/cobra"
)

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Показать последние события",
	RunE:  runEvents,
}

func runEvents(cmd *cobra.Command, args []string) error {
	snap, err := engine.ReadStateFile(runStateFile)
	if err != nil {
		fmt.Println("SmartRoute: демон не запущен.")
		fmt.Println("Запустите: smartroute run")
		return nil
	}
	fmt.Printf("SmartRoute: демон запущен (generation=%d, ready=%v)\n", snap.Generation, snap.Ready)
	fmt.Printf("События доступны в журнале демона. Используйте: smartroute logs\n")
	return nil
}
