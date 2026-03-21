package webserver

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/bslie/smartroute/internal/engine"
)

// APIPayload — ответ /api/v1/status.
type APIPayload struct {
	Snapshot *engine.StateSnapshot   `json:"snapshot"`
	History  []engine.MetricSample   `json:"history"`
}

// Start запускает HTTP-сервер Web UI (блокирует до ошибки Listen).
func Start(addr string, eng *engine.Engine) {
	if addr == "" {
		return
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		payload := APIPayload{
			Snapshot: nil,
			History:  nil,
		}
		if eng != nil {
			payload.Snapshot = eng.LatestSnapshot()
			payload.History = eng.MetricHistorySamples()
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
	fs := subFS()
	mux.Handle("/", http.FileServer(http.FS(fs)))

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("web ui: listening on http://%s/", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("web ui: %v", err)
	}
}
