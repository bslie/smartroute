package engine

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bslie/smartroute/internal/decision"
	"github.com/bslie/smartroute/internal/domain"
	"github.com/bslie/smartroute/internal/eventbus"
	"github.com/bslie/smartroute/internal/memlog"
	"github.com/bslie/smartroute/internal/metrics"
	"github.com/bslie/smartroute/internal/observer"
	"github.com/bslie/smartroute/internal/probe"
	"github.com/bslie/smartroute/internal/store"
)

// tunnelQuarantineState хранит данные о карантине туннеля.
type tunnelQuarantineState struct {
	quarantinedAt  time.Time
	cooldownUntil  time.Time
	recoveryProbes int // кол-во подряд успешных проб в cooldown
	backoffCount   int // кол-во повторных quarantine (для backoff cooldown)
}

// Engine — оркестратор: tick loop, решение, обновление store, отправка desired в reconciler.
type Engine struct {
	Store            *store.Store
	Bus              *eventbus.Bus
	MemLog           *memlog.Ring
	Reconciler       *Reconciler
	ConfigMu         *sync.RWMutex
	Config           **domain.Config
	ConfigGeneration *uint64 // счётчик reload конфига (обновляется из run.go)
	TickInterval     time.Duration
	StateFile        string // путь для дампа состояния (status без демона)
	GameModeFile     string // путь к файлу профиля (game/default) — читается каждый тик
	ConntrackPath    string // путь к /proc/net/nf_conntrack
	Ready            bool
	cancel           context.CancelFunc
	prevScores       map[string]float64 // tunnel name -> last health score для событий degraded/recovered
	degradedSince    map[string]time.Time
	quarantineState  map[string]*tunnelQuarantineState
	lastPassive      map[string]observer.PassiveStats
	lastProbe        map[string]domain.ProbeResult
	probeScheduler   *probe.Scheduler
	probePool        *probe.Pool
	probeCancel      context.CancelFunc
	dnsCache         *observer.DNSCache
}

// New создаёт engine.
func New(
	st *store.Store,
	bus *eventbus.Bus,
	ml *memlog.Ring,
	rec *Reconciler,
	cfgMu *sync.RWMutex,
	cfg **domain.Config,
	configGeneration *uint64,
) *Engine {
	conntrackPath := "/proc/net/nf_conntrack"
	return &Engine{
		Store:            st,
		Bus:              bus,
		MemLog:           ml,
		Reconciler:       rec,
		ConfigMu:         cfgMu,
		Config:           cfg,
		ConfigGeneration: configGeneration,
		TickInterval:     2 * time.Second,
		ConntrackPath:    conntrackPath,
		lastPassive:      make(map[string]observer.PassiveStats),
		lastProbe:        make(map[string]domain.ProbeResult),
		prevScores:       make(map[string]float64),
		degradedSince:    make(map[string]time.Time),
		quarantineState:  make(map[string]*tunnelQuarantineState),
		dnsCache:         observer.NewDNSCache(300 * time.Second),
	}
}

// Run запускает tick loop в горутине. Контекст для остановки.
func (e *Engine) Run(ctx context.Context) {
	ctx, e.cancel = context.WithCancel(ctx)
	e.MemLog.Write("info", "engine: tick loop started")
	currentInterval := e.TickInterval
	ticker := time.NewTicker(currentInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			e.MemLog.Write("info", "engine: tick loop stopped")
			return
		case <-ticker.C:
			e.tick(ctx)
			// Перезапускаем тикер если интервал изменился (hot-reload)
			if e.TickInterval > 0 && e.TickInterval != currentInterval {
				ticker.Reset(e.TickInterval)
				currentInterval = e.TickInterval
			}
		}
	}
}

// tick — один цикл: observe (conntrack), classify, decide, update store, trigger reconcile.
func (e *Engine) tick(ctx context.Context) {
	e.ConfigMu.RLock()
	cfg := *e.Config
	e.ConfigMu.RUnlock()
	if cfg == nil {
		return
	}
	e.Store.Lock()
	e.Store.Generation++
	if e.ConfigGeneration != nil {
		e.Store.ConfigGeneration = atomic.LoadUint64(e.ConfigGeneration)
	}
	// Синхронизация туннелей с конфигом (hot-reload: добавление/удаление туннелей).
	configNames := make(map[string]struct{}, len(cfg.Tunnels))
	for i, tc := range cfg.Tunnels {
		configNames[tc.Name] = struct{}{}
		tunnelIndex := uint32(i + 1)
		fw := tc.FWMark
		if fw == 0 {
			fw = tunnelIndex
		}
		rt := tc.RouteTable
		if rt == 0 {
			rt = 200 + int(tunnelIndex)
		}
		if e.Store.Tunnels.Get(tc.Name) == nil {
			t := &domain.Tunnel{
				Name: tc.Name, Endpoint: tc.Endpoint, Interface: "wg-" + tc.Name,
				RouteTable: rt, FWMark: fw, IsDefault: tc.IsDefault,
				State: domain.TunnelStateDeclared,
				Health: domain.TunnelHealth{Score: 1.0, Liveness: domain.LivenessUp},
			}
			e.Store.Tunnels.Set(t)
		}
	}
	for _, name := range e.Store.Tunnels.Names() {
		if _, inCfg := configNames[name]; !inCfg {
			e.Store.Tunnels.Delete(name)
			delete(e.quarantineState, name)
			delete(e.degradedSince, name)
			delete(e.prevScores, name)
			delete(e.lastPassive, "wg-"+name)
			// Очистка lastProbe: ключ "ip|tunnel"
			for k := range e.lastProbe {
				if strings.HasSuffix(k, "|"+name) {
					delete(e.lastProbe, k)
				}
			}
		}
	}
	if !e.Store.Ready {
		e.Store.Ready = true
		e.Store.ActiveProfile = readGameModeFile(e.GameModeFile)
		e.Bus.Send(domain.Event{Type: domain.EventSystemReady, Timestamp: time.Now(), Severity: domain.SeverityInfo, Message: "system ready"})
	}
	if e.GameModeFile != "" {
		if p := readGameModeFile(e.GameModeFile); p != "" && p != e.Store.ActiveProfile {
			e.Store.ActiveProfile = p
		}
	}
	e.ensureProbeSubsystem(cfg)
	e.updateTunnelHealthFromPassive()
	e.collectProbeResults()
	e.runTunnelStateMachine(cfg)
	// Observe → classify → decide: conntrack → destinations → classifier → decider → assignments
	e.runObserveDecideLoop(cfg)
	e.Store.AppliedGen = e.Store.Generation
	e.Store.AppliedConfigGen = e.Store.ConfigGeneration
	snap := BuildStateSnapshot(e.Store)
	// Снимаем Lock ДО вызова TriggerReconcile: reconcile выполняет exec.Command и может
	// занять сотни мс — нельзя держать Store.Lock() всё это время (CLI не сможет читать).
	e.Store.Unlock()
	e.Reconciler.TriggerReconcile(cfg, e.Store)
	WriteStateFileSafe(&snap, e.StateFile)
	// Убираем defer Store.Unlock() — мы уже разблокировали вручную.
}

// Stop останавливает tick loop.
func (e *Engine) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
	if e.probeCancel != nil {
		e.probeCancel()
	}
	if e.probePool != nil {
		e.probePool.Stop()
	}
}

// runObserveDecideLoop: conntrack → destinations → classify → decide → store (assignments).
func (e *Engine) runObserveDecideLoop(cfg *domain.Config) {
	// Подпитка DNS-кэша из лога dnsmasq (confidence 0.8 по доке)
	if cfg.DnsmasqLogPath != "" && e.dnsCache != nil {
		records, _ := observer.ReadDnsmasqLog(cfg.DnsmasqLogPath, 64*1024)
		for _, rec := range records {
			e.dnsCache.Set(rec.IP, rec.Domain, 0.8)
		}
	}
	entries, err := observer.ReadConntrack(e.ConntrackPath)
	if err != nil {
		return
	}
	tunnels := e.Store.Tunnels.All()
	defaultTunnel := decision.DefaultTunnelFromList(tunnels)
	if defaultTunnel == "" && len(tunnels) > 0 {
		defaultTunnel = tunnels[0].Name
	}
	classifier := &decision.Classifier{StaticRoutes: cfg.StaticRoutes}
	scorer := &decision.Scorer{
		SignalTTL:   cfg.Probe.SignalTTL,
		StickyBonus:  cfg.StickyBonus,
	}
	decider := &decision.Decider{
		Classifier:         classifier,
		Scorer:             scorer,
		StaticRoutes:       cfg.StaticRoutes,
		Tunnels:            tunnels,
		DefaultTunnel:      defaultTunnel,
		StickyCycles:       cfg.StickyCycles,
		HysteresisWebPct:  cfg.HysteresisWebPct,
		HysteresisBulkPct:  cfg.HysteresisBulkPct,
		HysteresisGamePct:  cfg.HysteresisGamePct,
		StickyBonus:        cfg.StickyBonus,
		StickyBonusGame:    200,
		ActiveProfile:      e.Store.ActiveProfile,
	}
	now := time.Now()
	seenIP := make(map[string]struct{})
	if e.probeScheduler != nil {
		e.probeScheduler.ResetTick()
	}
	for _, ent := range entries {
		if ent.DstIP == nil {
			continue
		}
		ipStr := ent.DstIP.String()
		if _, ok := seenIP[ipStr]; ok {
			continue
		}
		seenIP[ipStr] = struct{}{}
		dest := e.Store.Destinations.Get(ent.DstIP)
		if dest == nil {
			dest = &domain.Destination{
				IP:         ent.DstIP,
				Port:       ent.DstPort,
				Proto:      ent.Proto,
				Domain:     "",
				DomainConf: 0.1,
				Class:      domain.TrafficClassUnknown,
				State:      domain.DestStateDiscovered,
				LastSeen:   now,
				FirstSeen:  now,
			}
		} else {
			dest.LastSeen = now
			dest.Port = ent.DstPort
			dest.Proto = ent.Proto
		}
		// Обогащаем domain из DNS cache для лучшей классификации
		if dnsDomain, dnsConf := e.dnsCache.Get(dest.IP); dnsDomain != "" && dnsConf > dest.DomainConf {
			dest.Domain = dnsDomain
			dest.DomainConf = dnsConf
		}
		cr := classifier.Classify(dest.IP.String(), dest.Domain, dest.Port)
		dest.Class = cr.Class
		if cr.Confidence > dest.DomainConf {
			dest.DomainConf = cr.Confidence
		}
		latencyByTunnel := make(map[string]int)
		negativeByTunnel := make(map[string]float64)
		for _, t := range tunnels {
			latencyByTunnel[t.Name] = 1000 + t.Health.PenaltyMs
			negativeByTunnel[t.Name] = 1.0
			if pr, ok := e.lastProbe[probeKey(dest.IP, t.Name)]; ok {
				age := now.Sub(pr.Timestamp)
				if cfg.Probe.SignalTTL <= 0 || age <= cfg.Probe.SignalTTL {
					if pr.LatencyMs > 0 {
						latencyByTunnel[t.Name] = pr.LatencyMs + t.Health.PenaltyMs
					}
					negativeByTunnel[t.Name] = domain.NegativeSignalFactor(pr.ErrorClass)
				}
			}
		}
		assignment := decider.Decide(dest, latencyByTunnel, negativeByTunnel)
		prevAssignment := dest.Assignment
		dest.Assignment = assignment
		dest.State = domain.DestStateAssigned
		e.Store.Destinations.Set(dest)
		if assignment != nil {
			assignment.Generation = e.Store.ConfigGeneration
			e.Store.Assignments.Set(dest.IP, assignment)
			// Счётчик переключений туннеля
			if prevAssignment == nil || prevAssignment.TunnelName != assignment.TunnelName {
				metrics.IncAssignmentSwitches()
			}
			e.submitProbeJob(dest, assignment.TunnelName, cfg)
		}
	}
	// GC destinations: stale и expired по cfg.DestTTL (docs: dest_ttl 120s default)
	staleTTL := 60 * time.Second
	expiredTTL := cfg.DestTTL
	if expiredTTL <= 0 {
		expiredTTL = 120 * time.Second
	}
	if staleTTL > expiredTTL {
		staleTTL = expiredTTL / 2
	}
	for _, dest := range e.Store.Destinations.All() {
		if _, seen := seenIP[dest.IP.String()]; seen {
			continue
		}
		age := now.Sub(dest.LastSeen)
		if age > expiredTTL {
			e.Store.Destinations.Delete(dest.IP)
			e.Store.Assignments.Delete(dest.IP)
		} else if age > staleTTL && dest.State != domain.DestStateStale {
			dest.State = domain.DestStateStale
			e.Store.Destinations.Set(dest)
		}
	}
}

func (e *Engine) ensureProbeSubsystem(cfg *domain.Config) {
	if e.probeScheduler == nil {
		e.probeScheduler = probe.NewScheduler(cfg.Probe.MaxProbesPerTick, cfg.Probe.MaxProbesPerDestinationPerMin)
	}
	if e.probePool != nil {
		return
	}
	e.probePool = probe.NewPool(5, probe.RunProbe)
	ctx, cancel := context.WithCancel(context.Background())
	e.probeCancel = cancel
	e.probePool.Start(ctx)
}

func (e *Engine) submitProbeJob(dest *domain.Destination, tunnel string, cfg *domain.Config) {
	if e.probePool == nil || e.probeScheduler == nil || dest == nil || dest.IP == nil || tunnel == "" {
		return
	}
	if !e.probeScheduler.Allow(dest.IP.String()) {
		return
	}
	timeout := cfg.Probe.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	probeType := domain.ProbeTCP
	if cfg.Probe.HTTPCheck {
		probeType = domain.ProbeHTTP
	}
	_ = e.probePool.Submit(probe.Job{
		DestIP:  dest.IP,
		Tunnel:  tunnel,
		Iface:   "wg-" + tunnel,
		Type:    probeType,
		Timeout: timeout,
	})
}

func (e *Engine) collectProbeResults() {
	if e.probePool == nil {
		return
	}
	for {
		select {
		case r, ok := <-e.probePool.Results():
			if !ok {
				return
			}
			if r.DestIP == nil || r.Tunnel == "" {
				continue
			}
			IncProbe()
			if r.ErrorClass != domain.ErrorNone {
				IncProbeFailed()
			}
			e.lastProbe[probeKey(r.DestIP, r.Tunnel)] = r
		default:
			return
		}
	}
}

func (e *Engine) updateTunnelHealthFromPassive() {
	now := time.Now()
	for _, t := range e.Store.Tunnels.All() {
		t.Health.HandshakeAgeSec = -1
		t.Health.PacketLossPct = 0
		if ageSec, err := observer.ReadWGHandshake(t.Interface); err == nil && ageSec >= 0 {
			t.Health.HandshakeAgeSec = ageSec
		}
		ps, err := observer.ReadProcNetDev(t.Interface)
		if err != nil {
			t.Health.Liveness = domain.LivenessUnknown
			t.Health.Score = 0.3
			t.Health.PenaltyMs = penaltyForScore(t.Health.Score)
			t.Health.LastCheck = now
			e.Store.Tunnels.Set(t)
			continue
		}
		score := 1.0
		prev, ok := e.lastPassive[t.Interface]
		if ok {
			deltaErr := (ps.RXErrors - prev.RXErrors) + (ps.TXErrors - prev.TXErrors)
			deltaDrop := (ps.RXDrop - prev.RXDrop) + (ps.TXDrop - prev.TXDrop)
			deltaPackets := (ps.RXPackets - prev.RXPackets) + (ps.TXPackets - prev.TXPackets)
			if deltaPackets > 0 {
				lossPct := 100 * float64(deltaErr+deltaDrop) / float64(deltaPackets)
				if lossPct > 100 {
					lossPct = 100
				}
				t.Health.PacketLossPct = lossPct
				if lossPct > 10 {
					score -= 0.3
				} else if lossPct > 5 {
					score -= 0.15
				} else if lossPct > 1 {
					score -= 0.05
				}
			}
			if deltaErr > 0 {
				score -= 0.2
			}
			deltaBytes := (ps.RXBytes - prev.RXBytes) + (ps.TXBytes - prev.TXBytes)
			if deltaBytes == 0 && deltaPackets == 0 {
				score -= 0.1
			}
		}
		// Handshake age: > 3 min = плохо (spec: handshake < 3 min для liveness)
		if t.Health.HandshakeAgeSec >= 0 {
			if t.Health.HandshakeAgeSec > 180 {
				score -= 0.4
				t.Health.Liveness = domain.LivenessDown
			} else if t.Health.HandshakeAgeSec > 120 {
				score -= 0.2
			} else if t.Health.HandshakeAgeSec > 60 {
				score -= 0.05
			} else {
				t.Health.Liveness = domain.LivenessUp
			}
		}
		if score < 0.1 {
			score = 0.1
		}
		if t.Health.Liveness != domain.LivenessDown {
			t.Health.Liveness = domain.LivenessUp
		}
		t.Health.Score = score
		t.Health.PenaltyMs = penaltyForScore(score)
		t.Health.LastCheck = now
		e.Store.Tunnels.Set(t)
		e.lastPassive[t.Interface] = ps
	}
}

// runTunnelStateMachine обрабатывает переходы состояния туннеля:
// Active → Degraded → Quarantined → (cooldown) → Active.
// Параметры из спецификации §8:
//
//	degraded >= degradedTicksBeforeQuarantine тиков подряд → quarantine
//	quarantine cooldown = 30s*2^backoff (cap 240s)
//	recovery: 3 тика подряд с исправным health после окончания cooldown
const (
	degradedTicksBeforeQuarantine = 3
	baseCooldownDuration          = 30 * time.Second
	maxCooldownDuration           = 240 * time.Second
	recoveryTicksNeeded           = 3
)

func (e *Engine) runTunnelStateMachine(cfg *domain.Config) {
	_ = cfg
	now := time.Now()
	for _, t := range e.Store.Tunnels.All() {
		score := t.Health.Score
		isHealthy := score >= 0.8

		qs := e.quarantineState[t.Name]
		if qs == nil {
			qs = &tunnelQuarantineState{}
			e.quarantineState[t.Name] = qs
		}

		switch t.State {
		case domain.TunnelStateActive, domain.TunnelStateDeclared:
			if !isHealthy {
				if _, hasSince := e.degradedSince[t.Name]; !hasSince {
					e.degradedSince[t.Name] = now
					t.State = domain.TunnelStateDegraded
					e.Bus.Send(domain.Event{
						Type:      domain.EventTunnelDegraded,
						Timestamp: now,
						Severity:  domain.SeverityWarning,
						Tunnel:    t.Name,
						Message:   fmt.Sprintf("health score %.2f < 0.80", score),
					})
				}
			} else {
				if prev, ok := e.prevScores[t.Name]; ok && prev < 0.8 {
					delete(e.degradedSince, t.Name)
					e.Bus.Send(domain.Event{
						Type:      domain.EventTunnelRecovered,
						Timestamp: now,
						Severity:  domain.SeverityInfo,
						Tunnel:    t.Name,
						Message:   "health recovered",
					})
				}
				t.State = domain.TunnelStateActive
			}

		case domain.TunnelStateDegraded:
			if isHealthy {
				delete(e.degradedSince, t.Name)
				t.State = domain.TunnelStateActive
				e.Bus.Send(domain.Event{
					Type:      domain.EventTunnelRecovered,
					Timestamp: now,
					Severity:  domain.SeverityInfo,
					Tunnel:    t.Name,
					Message:   "health recovered from degraded",
				})
			} else {
				if since, ok := e.degradedSince[t.Name]; ok {
					degradedTicks := int(now.Sub(since) / e.TickInterval)
					if degradedTicks >= degradedTicksBeforeQuarantine {
						cooldown := baseCooldownDuration * (1 << minInt(qs.backoffCount, 3))
						if cooldown > maxCooldownDuration {
							cooldown = maxCooldownDuration
						}
						qs.quarantinedAt = now
						qs.cooldownUntil = now.Add(cooldown)
						qs.recoveryProbes = 0
						qs.backoffCount++
						delete(e.degradedSince, t.Name)
						t.State = domain.TunnelStateQuarantined
						t.Health.Liveness = domain.LivenessDown
						metrics.IncTunnelDegraded()
						e.Bus.Send(domain.Event{
							Type:      domain.EventTunnelDegraded,
							Timestamp: now,
							Severity:  domain.SeverityCritical,
							Tunnel:    t.Name,
							Message:   fmt.Sprintf("quarantined for %s (backoff #%d)", cooldown, qs.backoffCount),
						})
					}
				}
			}

		case domain.TunnelStateQuarantined:
			if now.After(qs.cooldownUntil) {
				if isHealthy {
					qs.recoveryProbes++
					if qs.recoveryProbes >= recoveryTicksNeeded {
						qs.backoffCount = 0
						qs.recoveryProbes = 0
						t.State = domain.TunnelStateActive
						t.Health.Liveness = domain.LivenessUp
						e.Bus.Send(domain.Event{
							Type:      domain.EventTunnelRecovered,
							Timestamp: now,
							Severity:  domain.SeverityInfo,
							Tunnel:    t.Name,
							Message:   "recovered from quarantine after cooldown",
						})
					}
				} else {
					qs.recoveryProbes = 0
					cooldown := baseCooldownDuration * (1 << minInt(qs.backoffCount, 3))
					if cooldown > maxCooldownDuration {
						cooldown = maxCooldownDuration
					}
					qs.quarantinedAt = now
					qs.cooldownUntil = now.Add(cooldown)
					qs.backoffCount++
					e.Bus.Send(domain.Event{
						Type:      domain.EventTunnelDegraded,
						Timestamp: now,
						Severity:  domain.SeverityCritical,
						Tunnel:    t.Name,
						Message:   fmt.Sprintf("still unhealthy, extending quarantine (backoff #%d)", qs.backoffCount),
					})
				}
			}
		}

		e.prevScores[t.Name] = score
		e.Store.Tunnels.Set(t)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func penaltyForScore(score float64) int {
	switch {
	case score >= 0.8:
		return 0
	case score >= 0.5:
		return 50
	case score >= 0.3:
		return 150
	default:
		return 500
	}
}

func probeKey(ip interface{ String() string }, tunnel string) string {
	return fmt.Sprintf("%s|%s", ip.String(), tunnel)
}

func readGameModeFile(path string) string {
	if path == "" {
		return "default"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "default"
	}
	p := strings.TrimSpace(string(data))
	if p == "game" {
		return "game"
	}
	return "default"
}
