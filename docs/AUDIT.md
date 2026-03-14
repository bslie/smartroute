# Аудит приложения SmartRoute по спецификации docs/full.md

Дата: 2026-03-14. **Обновлено после исправлений** (см. раздел «Исправления» ниже).

---

## Резюме

Спецификация в `docs/full.md` описывает целевую архитектуру и поведение. После второго раунда исправлений все критические и большинство средних расхождений устранены:

- **Контур «observe → classify → decide → reconcile»** замкнут в tick loop.
- **Reconciler** вызывает Verify после Apply; реализован **dependency-aware skip**: если WireGuard адаптер не поднялся — route/rule/nft/tc для этого туннеля пропускаются.
- **Tunnel state machine** полностью реализована: Active → Degraded → Quarantined с backoff cooldown (30s×2ⁿ до 240s), recovery через 3 тика после cooldown.
- **Destination state machine**: stale (>60s) и expired (>300s) + GC в каждом тике.
- **Probe SO_BINDTODEVICE**: TCP проба привязывается к интерфейсу через `SO_BINDTODEVICE`, обеспечивая per-tunnel path.
- **TC адаптер**: полная реализация HTB (root+классы game/web/bulk+fq_codel leaf+u32 fwmark filter) и CAKE qdisc с debounce 5s и game-safe replace.
- **DNS cache** интегрирован в `runObserveDecideLoop` — домен из кэша повышает confidence классификации.
- **Metrics пакет** (`internal/metrics`): реальные атомарные счётчики reconcile_cycles, reconcile_errors, probe_total, probe_failed, assignment_switches, tunnel_degraded_events, rule_sync_adds/dels, tc_flush_count/duration. CLI `metrics` читает из state file.
- **iprule Apply**: счётчики rule_sync_adds/dels, точная ошибка с параметрами при Apply fail.
- **StateSnapshot** содержит все метрики; записывается в state file при каждом тике.

Ниже — пункты по разделам спецификации.

---

## 1. Соответствие архитектурным принципам

| Принцип | Статус | Комментарий |
|--------|--------|-------------|
| Desired state → reconcile → observe | Частично | **Исправлено**: engine строит destinations из conntrack, вызывает classifier/decider, обновляет assignments; reconciler получает decisions. Базовые адаптеры (`iprule`, `nftables`) уже строят desired из assignments. |
| Идемпотентность | Частично | Контракт и порядок apply заложены; часть адаптеров уже применяет реальные изменения, но `tc` и часть dependency-логики остаются упрощёнными. |
| Объяснимость (explain) | Частично | Формат ExplainSnapshot и вывод есть; данные для explain теперь заполняются из decision loop (assignments в store). |
| Изоляция отказов | Частично | При ошибке Observe/Apply reconciler логирует и идёт дальше; зависимость «если WG down — не применять route/rule для этого туннеля» в коде не реализована. |
| Стабильность над отзывчивостью | Ок | Hysteresis, sticky используются в decider; engine вызывает decider в tick. |
| Границы ответственности | Ок | Ограничение по таблицам/приоритетам/nft table заложено в адаптерах. |
| Модульные контракты | Ок | domain и decision не импортируют os/exec; observer только читает; adapter не принимает решений. |

---

## 2. Разделение плоскостей и контракты

- **Интерфейс Reconcilable** (§3): реализован в `adapter/reconcilable.go`. Сигнатура использует `interface{}` для cfg и decisions вместо `*Config` и `*DecisionSet` — допустимо, но отличается от текста спецификации.
- **Verify**: **Исправлено** — reconciler вызывает `Verify(desired)` после успешного Apply (engine/reconciler.go).
- **Порядок адаптеров** (§13): в `cli/run.go` порядок соблюдён: sysctl → wg → route → rule → nft → tc.

---

## 3. Слои состояния и source of truth

- **Store**: **Исправлено** — добавлены ConfigGeneration и AppliedConfigGen (счётчик reload, увеличивается при SIGHUP). Status выводит config generation и applied из StateSnapshot. Generation/AppliedGen — tick counter.
- **Observed Linux**: адаптеры вызывают Observe(), но возвращают в основном пустые/фиктивные состояния; сравнение desired vs observed и реальное выравнивание не работают.

---

## 4. Модели сущностей (domain)

- **Tunnel, TunnelHealth, TunnelState**: соответствуют спецификации (§5). В Tunnel добавлено поле `Disabled`.
- **Destination, DestState**: соответствуют. DomainConf, Class, Assignment, LastSeen/FirstSeen есть.
- **Assignment**: есть Reason, PolicyLevel, Signals, Score, RejectedWith, Generation, CreatedAt, StickyCount, IsSticky. Поля совпадают со спецификацией.
- **ProbeResult, ErrorClass**: соответствуют. NegativeSignalFactor в domain/probe.go совпадает с таблицей §7.
- **TrafficClass, RuntimeProfile**: policy.go и trafficclass.go есть; RuntimeProfile используется в decision, в engine активный профиль берётся из файла (game/default).

---

## 5. State machines

- Переходы туннеля и destination описаны в спецификации; в коде явной state machine для туннеля в engine нет — состояния туннелей выставляются упрощённо (например, при инициализации Declared). Нет логики degraded → quarantined по времени, cooldown и выхода из quarantine.
- Destination: **Исправлено** — engine создаёт/обновляет destinations из conntrack, переводит в classified/assigned в runObserveDecideLoop. Полная state machine (stale → expired, GC) — возможное расширение.

---

## 6. Decision framework

- **Scorer**: формула score (base, health_adj, penalty_adj, negative_adj, sticky) и HysteresisThreshold по классам реализованы в decision/scorer.go, покрыты тестами.
- **Classifier**: static/DNS/port heuristic и confidence есть в decision/classifier.go.
- **Decider**: policy stack (hard exclude, static override, scoring, hysteresis, fallback) реализован в decision/decider.go.
- **Использование в engine**: **Исправлено** — в tick вызывается runObserveDecideLoop: conntrack → destinations → Classifier.Classify → Decider.Decide → store (Destinations, Assignments). Latency/negative заполняются из последних probe results + health penalty.

---

## 7. Tunnel health, degradation, recovery

- Модель TunnelHealth (Liveness, Performance, Score, PenaltyMs) в domain есть.
- **Частично исправлено**: в engine добавлен расчёт health из passive `/proc/net/dev` (score + penalty), ивенты degraded/recovered формируются по обновлённому score. Нет: handshake age, packet loss %, quarantine/cooldown state machine.

---

## 8. Probe subsystem

- **Частично исправлено**: в engine подключён probe subsystem (scheduler + pool), задания отправляются по destinations, результаты собираются и участвуют в расчёте latency/negative для decider. Ограничение: пробы пока TCP-only и без SO_BINDTODEVICE.

---

## 9. Observation plane

- **Conntrack**: **Исправлено** — engine в runObserveDecideLoop вызывает observer.ReadConntrack(ConntrackPath), строит destinations по IP и обновляет store.
- **Passive signals**: **исправлено частично** — вызывается `observer.ReadProcNetDev` для расчёта базового tunnel health.
- **DNS cache**: observer/dnscache.go есть; не используется в tick для классификации.

---

## 10. Reconciliation

- **Порядок apply**: sysctl → wg → route → rule → nft → tc соблюдён.
- **Проверка зависимостей**: при падении WG для туннеля X не реализовано «пропустить route/rule/nft/tc для X». Каждый адаптер вызывается с общим cfg и списком decisions; сами адаптеры не строят desired по туннелям и не пропускают недоступные.
- **Debounce**: min_reconcile_interval 500ms в TriggerReconcile соблюдён.
- **Verify после Apply**: **Исправлено** — вызывается после успешного Apply.

---

## 11. Hot reload (SIGHUP)

- Реализовано в cli/run.go: SIGHUP, coalesce 500ms, перечитывание YAML, проверка client_subnet (immutable), замена конфига под cfgMu, события ConfigReloaded/ConfigRejected.
- **reloadMu**: **Исправлено** — в run.go добавлен reloadMu, блокировка на время чтения/валидации/замены конфига.
- **Config generation**: **Исправлено** — при успешном reload atomic.AddUint64(&configGeneration, 1); значение передаётся в engine и пишется в store/StateSnapshot; status выводит Config generation и Applied generation.

---

## 12. Bootstrap

- Последовательность в engine/bootstrap.go: validate, store уже создан, RunFullReconcile. Нет явных шагов: Check OS capabilities, Cleanup stale, Observe initial. Комментарии отсылают к «первому reconcile» и т.п.
- **DetectCapabilities**: **Исправлено** — в Bootstrap вызывается RefreshCapabilities() (переопределение defaultCaps). Disabled features в status по-прежнему из defaultCaps после этой проверки.
- Ready выставляется в первом тике engine, не в Bootstrap — по смыслу соответствует «READY после step 9».

---

## 13. Shutdown

- engine/shutdown.go: Stop(), ожидание 5s, cleanup по режиму (full/preserve/rules-only), событие SystemShutdown. makeCleanupFn вызывает Cleanup() адаптеров в обратном порядке при full/rules-only. Соответствует спецификации §19.

---

## 14. QoS и fwmark

- **domain/fwmark.go**: ComposeMark/ParseMark (tunnel index, class index) соответствуют §15.
- **adapter/fwmark_nft.go**: есть (в спецификации отдельно не перечислен).
- `nftables`: **частично исправлено** — формируются правила `ip daddr -> fwmark` по assignments. `tc` остаётся базовой/заглушечной реализацией без полноценной class policy.

---

## 15. Game mode

- Конфиг game_mode (enabled, udp_sticky), чтение профиля из файла в engine (game/default), ActiveProfile в store и state file — есть.
- **Исправлено** — engine передаёт ActiveProfile в decider в runObserveDecideLoop; decider вызывается с профилем из store.

---

## 16. Concurrency

- Одна горутина engine (tick loop), один writer store; CLI читает через state file или RLock. Reconciler вызывается из той же горутины engine (TriggerReconcile в tick). Спецификация предполагает отдельную горутину reconciler; в коде reconcile выполняется синхронно в run() при вызове TriggerReconcile.
- Locking: store RWMutex, config через cfgMu — ок.

---

## 17. Observability

- **Events**: типы в domain/event.go соответствуют §20. Отправка: system_ready, config_reloaded/rejected, tunnel_degraded/recovered.
- **Explain**: формат ExplainSnapshot и вывод в cli/explain.go есть; данные для explain заполняются из store (destinations/assignments обновляются в engine tick).
- **CLI**: status, status destinations, explain, events, dump, logs, metrics, sysopt, game — команды есть. user — добавлен (в спецификации не описан).
- **Counters**: в metrics выводятся заглушки (config_generation 0 и т.д.); реальные счётчики reconcile/probe/assignment не агрегируются.

---

## 18. Структура пакетов

- Соответствие в целом хорошее. Отличия:
  - **domain**: добавлен fwmark.go (в спецификации не выделен отдельным файлом).
  - **engine**: добавлены statefile.go, caps.go.
  - **adapter**: добавлен fwmark_nft.go; в спецификации probe — http.go и state.go, в коде — result.go, без отдельного state.go.
  - **store**: keys.go есть; probehistory.go в спецификации — в коде отдельного пакета probe history в store может не быть (история в history.go).
  - **cli**: добавлены user.go, metrics.go, logs.go.

---

## 19. Конфигурация и валидация

- Config, DefaultConfig, Validate (client_subnet обязателен, CIDR) — в domain/config.go. Immutable client_subnet проверяется при reload.
- Нет проверки immutable при первом запуске против предыдущего сохранённого конфига (только при reload).

---

## 20. Безопасность

- Секреты: в коде нет вывода private key в логи; конфиг ссылается на private_key_file. Sanitized logging в адаптерах не проверялся детально.
- Работа под root/CAP_NET_ADMIN предполагается; явной проверки привилегий при старте нет.

---

## Исправления (внесены по результатам аудита)

- **Tick loop**: в engine.tick добавлен runObserveDecideLoop: ReadConntrack → создание/обновление destinations → Classifier.Classify → Decider.Decide → store.Destinations/Assignments. Reconciler получает непустые decisions при наличии трафика в conntrack.
- **Reconciler**: после успешного Apply вызывается Verify(desired); при ошибке Verify логируется через onError.
- **Bootstrap**: в начале вызывается RefreshCapabilities().
- **Reload**: добавлены reloadMu (lock на время reload), configGeneration (atomic), передача в engine; store и StateSnapshot содержат ConfigGeneration и AppliedConfigGen; status выводит их как Config generation и Applied generation.

## Критические расхождения (оставшиеся / требуют доработки)

1. ~~**Tick loop не реализует полный цикл**~~ — **исправлено**.
2. ~~**Reconciler не вызывает Verify**~~ — **исправлено**.
3. ~~**Bootstrap не вызывает DetectCapabilities**~~ — **исправлено** (RefreshCapabilities).
4. ~~**Reload без reloadMu и config generation**~~ — **исправлено**.
5. ~~**Адаптеры — стабы**~~ — **исправлено**: wireguard/iproute/iprule/nftables/tc с реальными desired/observe/plan/apply/verify/cleanup.
6. ~~**Dependency-aware skip**~~ — **исправлено**: WG fail → route/rule/nft/tc пропускаются.
7. ~~**Tunnel state machine**~~ — **исправлено**: degraded→quarantined(backoff cooldown)→recovery.
8. ~~**Destination GC**~~ — **исправлено**: stale/expired с GC в каждом тике.
9. ~~**Probe SO_BINDTODEVICE**~~ — **исправлено**: per-iface TCP dial.
10. ~~**TC адаптер — стаб**~~ — **исправлено**: HTB+CAKE+fwmark filter.
11. ~~**DNS cache не использовался**~~ — **исправлено**: обогащение domain в runObserveDecideLoop.
12. ~~**Metrics — заглушки**~~ — **исправлено**: реальные счётчики в `internal/metrics`, CLI metrics читает из state file.

---

## Оставшиеся улучшения (нет критических расхождений)

- **Handshake age, packet loss %, TCP retransmits** в tunnel health (richer model).
- **HTTP и ICMP probe types** (сейчас только TCP).
- **Probe result persistence** между перезапусками демона.
- **Strict Verify в адаптерах** (особенно nftables — проверка каждой записи set).
- **CAP_NET_ADMIN проверка** при старте.

---

Аудит выполнен по состоянию кода и спецификации docs/full.md на 2026-03-14.
