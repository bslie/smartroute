package eventbus

import (
	"sync"

	"github.com/bslie/smartroute/internal/domain"
)

// Bus — шина событий с кольцевым буфером.
type Bus struct {
	ch      chan domain.Event
	ring    []domain.Event
	ringMu  sync.RWMutex
	ringCap int
	ringIdx int
}

// New создаёт шину с буфером канала и кольцом последних событий.
func New(chCap, ringCap int) *Bus {
	if chCap <= 0 {
		chCap = 64
	}
	if ringCap <= 0 {
		ringCap = 200
	}
	return &Bus{
		ch:      make(chan domain.Event, chCap),
		ring:    make([]domain.Event, ringCap),
		ringCap: ringCap,
	}
}

// Send отправляет событие (неблокирующе; при переполнении канала пропускаем).
func (b *Bus) Send(e domain.Event) {
	select {
	case b.ch <- e:
	default:
	}
	b.ringMu.Lock()
	b.ring[b.ringIdx] = e
	b.ringIdx = (b.ringIdx + 1) % b.ringCap
	b.ringMu.Unlock()
}

// C возвращает канал для приёма событий.
func (b *Bus) C() <-chan domain.Event {
	return b.ch
}

// Last возвращает последние события из кольца (новые в конце).
func (b *Bus) Last(n int) []domain.Event {
	b.ringMu.RLock()
	defer b.ringMu.RUnlock()
	if n > b.ringCap {
		n = b.ringCap
	}
	out := make([]domain.Event, 0, n)
	for i := 0; i < n; i++ {
		pos := (b.ringIdx - 1 - i + b.ringCap*2) % b.ringCap
		ev := b.ring[pos]
		if ev.Type == "" {
			continue
		}
		out = append(out, ev)
	}
	return out
}
