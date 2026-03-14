package engine

import (
	"context"
	"time"

	"github.com/bslie/smartroute/internal/domain"
	"github.com/bslie/smartroute/internal/eventbus"
)

// Shutdown выполняет остановку: cancel tick, wait reconcile, cleanup по режиму.
func Shutdown(
	ctx context.Context,
	engine *Engine,
	bus *eventbus.Bus,
	cleanupMode string,
	adaptersCleanup func(mode string),
) {
	engine.Stop()
	// Ждём завершения текущего reconcile (упрощённо — таймаут 5s)
	select {
	case <-time.After(5 * time.Second):
	case <-ctx.Done():
	}
	if cleanupMode == "" {
		cleanupMode = "full"
	}
	adaptersCleanup(cleanupMode)
	bus.Send(domain.Event{Type: domain.EventSystemShutdown, Timestamp: time.Now(), Severity: domain.SeverityInfo, Message: "system shutdown"})
}
