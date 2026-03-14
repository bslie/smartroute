package cli

import (
	"fmt"
	"time"

	"github.com/smartroute/smartroute/internal/eventbus"
	"github.com/spf13/cobra"
)

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Показать последние события",
	RunE:  runEvents,
}

func runEvents(cmd *cobra.Command, args []string) error {
	bus := eventbus.New(0, 200)
	evs := bus.Last(50)
	for _, e := range evs {
		fmt.Printf("%s [%s] %s %s\n", e.Timestamp.Format(time.RFC3339), e.Severity, e.Type, e.Message)
	}
	return nil
}
