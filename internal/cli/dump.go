package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bslie/smartroute/internal/engine"
	"github.com/bslie/smartroute/internal/store"
	"github.com/spf13/cobra"
)

var dumpStateFile string

var dumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Полный runtime state в JSON",
	RunE:  runDump,
}

func init() {
	dumpCmd.Flags().StringVar(&dumpStateFile, "state-file", "/var/run/smartroute/state.json", "файл состояния демона")
}

func runDump(cmd *cobra.Command, args []string) error {
	snap, err := engine.ReadStateFile(dumpStateFile)
	if err == nil {
		data, err := json.MarshalIndent(snap, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	// Демон не запущен — выводим пустой state из локального store (для совместимости)
	st := store.New()
	st.RLock()
	defer st.RUnlock()
	m := map[string]interface{}{
		"tunnels":      st.Tunnels.Names(),
		"destinations": st.Destinations.Count(),
		"ready":        st.Ready,
		"generation":   st.Generation,
	}
	data, _ := json.MarshalIndent(m, "", "  ")
	fmt.Println(string(data))
	fmt.Fprintf(os.Stderr, "Демон не запущен (файл %s не найден). Запустите: smartroute run\n", dumpStateFile)
	return nil
}
