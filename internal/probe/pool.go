package probe

import (
	"context"
	"sync"
)

// Pool — пул воркеров для проб. Adaptive: размер по загрузке (упрощённо — фиксированный).
type Pool struct {
	workers int
	jobs    chan Job
	results chan Result
	done    chan struct{}
	wg      sync.WaitGroup
	probeFn func(host, iface string, timeout interface{}) Result
}

// NewPool создаёт пул. probeFn инжектируется (для тестов — mock).
func NewPool(workers int, probeFn func(host, iface string, timeout interface{}) Result) *Pool {
	if workers <= 0 {
		workers = 5
	}
	p := &Pool{
		workers: workers,
		jobs:   make(chan Job, 100),
		results: make(chan Result, 100),
		done:   make(chan struct{}),
		probeFn: probeFn,
	}
	return p
}

// Start запускает воркеры.
func (p *Pool) Start(ctx context.Context) {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(ctx)
	}
}

// worker обрабатывает задания.
func (p *Pool) worker(ctx context.Context) {
	defer p.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case j, ok := <-p.jobs:
			if !ok {
				return
			}
			var timeout interface{} = j.Timeout
			r := p.probeFn(j.DestIP.String(), j.Iface, timeout)
			r.DestIP = j.DestIP
			r.Tunnel = j.Tunnel
			r.Type = j.Type
			select {
			case p.results <- r:
			case <-ctx.Done():
				return
			}
		}
	}
}

// Submit отправляет задание (неблокирующе).
func (p *Pool) Submit(j Job) bool {
	select {
	case p.jobs <- j:
		return true
	default:
		return false
	}
}

// Results возвращает канал результатов.
func (p *Pool) Results() <-chan Result {
	return p.results
}

// Stop останавливает пул.
func (p *Pool) Stop() {
	close(p.jobs)
	p.wg.Wait()
	close(p.results)
}
