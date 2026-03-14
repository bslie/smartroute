package cli

import (
	"fmt"

	"github.com/bslie/smartroute/internal/adapter"
	"github.com/spf13/cobra"
)

const sysoptBackupPath = "/var/run/smartroute/sysctl_backup.json"

var (
	sysoptDryRun   bool
	sysoptBackup   bool
	sysoptRollback bool
)

var sysoptCmd = &cobra.Command{
	Use:   "sysopt",
	Short: "OS tuning (sysctl): dry-run, backup, rollback",
}

var sysoptApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Применить sysctl из allowlist",
	RunE:  runSysoptApply,
}

var sysoptRollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Восстановить из backup",
	RunE:  runSysoptRollback,
}

func init() {
	sysoptCmd.AddCommand(sysoptApplyCmd)
	sysoptCmd.AddCommand(sysoptRollbackCmd)
	sysoptApplyCmd.Flags().BoolVar(&sysoptDryRun, "dry-run", false, "показать diff без применения")
	sysoptApplyCmd.Flags().BoolVar(&sysoptBackup, "backup", true, "сохранить backup перед применением")
	sysoptRollbackCmd.Flags().BoolVar(&sysoptRollback, "rollback", false, "выполнить откат")
}

func runSysoptApply(cmd *cobra.Command, args []string) error {
	a := adapter.NewSysctlAdapter(nil)
	if sysoptBackup {
		a.BackupPath = sysoptBackupPath
	}
	desired := a.Desired(nil, nil)
	observed, err := a.Observe()
	if err != nil {
		return err
	}
	diff := a.Plan(desired, observed)
	if sysoptDryRun {
		fmt.Println("dry-run: would apply sysctl diff (allowlist)")
		fmt.Printf("  desired: %v\n", desired)
		fmt.Printf("  observed: %v\n", observed)
		fmt.Printf("  diff: %v\n", diff)
		return nil
	}
	if sysoptBackup {
		fmt.Println("backup: saving current values to", sysoptBackupPath)
	}
	return a.Apply(diff)
}

func runSysoptRollback(cmd *cobra.Command, args []string) error {
	a := adapter.NewSysctlAdapter(nil)
	a.BackupPath = sysoptBackupPath
	if err := a.Restore(); err != nil {
		return fmt.Errorf("rollback: %w", err)
	}
	fmt.Println("rollback: restored sysctl from", sysoptBackupPath)
	return nil
}
