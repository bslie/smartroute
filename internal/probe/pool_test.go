package probe

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/smartroute/smartroute/internal/domain"
)

func TestPool_SubmitAndResult(t *testing.T) {
	results := make(chan Result, 2)
		probeFn := func(host, iface string, timeout interface{}) Result {
			return Result{
				DestIP:     nil,
			Tunnel:     "",
			Type:       domain.ProbeTCP,
			LatencyMs:  10,
			ErrorClass: domain.ErrorNone,
			Confidence: 0.9,
				Timestamp:  time.Now(),
			}
		}
	pool := NewPool(1, probeFn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pool.Start(ctx)
	defer pool.Stop()
	ok := pool.Submit(Job{
		DestIP:  net.ParseIP("1.2.3.4"),
		Tunnel:  "ams",
		Iface:   "wg-ams",
		Type:    domain.ProbeTCP,
		Timeout: time.Second,
	})
	if !ok {
		t.Fatal("Submit failed")
	}
	select {
	case r := <-pool.Results():
		if r.LatencyMs != 10 {
			t.Errorf("LatencyMs = %d", r.LatencyMs)
		}
	case <-time.After(time.Second):
		t.Fatal("no result")
	}
}
