package cli

import (
	"github.com/spf13/cobra"
)

// RootCmd — корневая команда smartroute.
var RootCmd = &cobra.Command{
	Use:   "smartroute",
	Short: "SmartRoute — операционная сетевая система",
	Long:  "Управление маршрутизацией через туннели WireGuard с desired-state reconciliation.",
}

func init() {
	RootCmd.AddCommand(runCmd)
	RootCmd.AddCommand(statusCmd)
	RootCmd.AddCommand(tunnelCmd)
	RootCmd.AddCommand(explainCmd)
	RootCmd.AddCommand(eventsCmd)
	RootCmd.AddCommand(dumpCmd)
	RootCmd.AddCommand(logsCmd)
	RootCmd.AddCommand(metricsCmd)
	RootCmd.AddCommand(sysoptCmd)
	RootCmd.AddCommand(gameCmd)
}
