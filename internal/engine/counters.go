package engine

import "github.com/bslie/smartroute/internal/metrics"

// Алиасы счётчиков engine для удобства (делегируют в metrics пакет).

func IncProbe()          { metrics.IncProbe() }
func IncProbeFailed()    { metrics.IncProbeFailed() }
func IncTunnelDegraded() { metrics.IncTunnelDegraded() }
