package probe

import (
	"sync"
	"time"

	"github.com/smartroute/smartroute/internal/domain"
)

// Scheduler — бюджет и rate limit проб. Решает, какие пробы запускать в тике.
type Scheduler struct {
	mu           sync.Mutex
	maxPerTick   int
	maxPerDestPerMin int
	usedThisTick int
	destCount    map[string]int
	lastMin      time.Time
}

// NewScheduler создаёт планировщик с лимитами.
func NewScheduler(maxPerTick, maxPerDestPerMin int) *Scheduler {
	if maxPerTick <= 0 {
		maxPerTick = 50
	}
	if maxPerDestPerMin <= 0 {
		maxPerDestPerMin = 6
	}
	return &Scheduler{
		maxPerTick:        maxPerTick,
		maxPerDestPerMin:  maxPerDestPerMin,
		destCount:         make(map[string]int),
		lastMin:           time.Now(),
	}
}

// Allow возвращает true, если можно отправить пробу для dest в этом тике.
func (s *Scheduler) Allow(destIP string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if now.Sub(s.lastMin) > time.Minute {
		s.destCount = make(map[string]int)
		s.lastMin = now
		s.usedThisTick = 0
	}
	if s.usedThisTick >= s.maxPerTick {
		return false
	}
	if s.destCount[destIP] >= s.maxPerDestPerMin {
		return false
	}
	s.usedThisTick++
	s.destCount[destIP]++
	return true
}

// ResetTick вызывается в начале каждого тика.
func (s *Scheduler) ResetTick() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.usedThisTick = 0
}

// ProbeResult alias
type ProbeResult = domain.ProbeResult
