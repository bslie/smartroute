package cli

import (
	"encoding/json"
	"fmt"

	"github.com/smartroute/smartroute/internal/store"
	"github.com/spf13/cobra"
)

var dumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Полный runtime state в JSON",
	RunE:  runDump,
}

func runDump(cmd *cobra.Command, args []string) error {
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
	return nil
}
