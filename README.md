# SmartRoute

Операционная сетевая система: маршрутизация трафика через туннели WireGuard с desired-state reconciliation, пробами, QoS и игровым режимом.

**Требования:** Go 1.21+, Linux (ip, wg, nft, tc, conntrack — по возможностям).

---

## Установка

### Вариант 1: из репозитория (скрипт)

Клонирование, сборка и установка в `/usr/local`:

```bash
git clone https://github.com/bslie/smartroute.git
cd smartroute
./scripts/install.sh
```

Переменные окружения (по желанию):

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `SMARTROUTE_REPO` | URL репозитория | URL для клонирования |
| `SMARTROUTE_VERSION` | `master` | Ветка или тег |
| `SMARTROUTE_INSTALL_DIR` | `/usr/local` | Каталог установки |
| `SMARTROUTE_BUILD_DIR` | `/tmp/smartroute-build` | Каталог сборки |
| `SMARTROUTE_SOURCE_DIR` | — | Если задан — сборка из этой папки без clone |
| `SKIP_INSTALL=1` | — | Только собрать, не устанавливать в INSTALL_DIR |

Пример: сборка из текущей папки без клонирования:

```bash
export SMARTROUTE_SOURCE_DIR="$(pwd)"
./scripts/install.sh
```

### Вариант 2: вручную (Makefile)

```bash
git clone https://github.com/bslie/smartroute.git
cd smartroute
make build
sudo make install
```

Бинарник окажется в `/usr/local/bin/smartroute`, пример конфига — в `/etc/smartroute/config.example.yaml`.

---

## Конфигурация

1. Скопируйте пример конфига и отредактируйте под свои туннели и маршруты:

```bash
sudo mkdir -p /etc/smartroute
sudo cp /usr/local/share/smartroute/config.example.yaml /etc/smartroute/config.yaml
# либо из дерева проекта:
sudo cp configs/smartroute.example.yaml /etc/smartroute/config.yaml
sudo nano /etc/smartroute/config.yaml
```

2. Обязательно задайте:
   - `client_subnet` — подсеть клиентов (CIDR)
   - `tunnels` — список туннелей WireGuard (name, endpoint, private_key_file, route_table, fwmark и т.д.)
   - при необходимости `static_routes`, `probe`, `game_mode`, `qos`

3. Файлы ключей WireGuard положите в безопасное место (например `/etc/smartroute/keys/`) и укажите пути в `private_key_file`. В Git не должны попадать `*.key` и `config.yaml` — они в `.gitignore`.

---

## Запуск

Запуск демона (читает конфиг из `/etc/smartroute/config.yaml`):

```bash
smartroute run
```

С указанием конфига и файла состояния:

```bash
smartroute run -c /etc/smartroute/config.yaml --state-file /var/run/smartroute/state.json
```

Демон работает до получения SIGTERM. По SIGHUP конфиг перечитывается (без смены `client_subnet`).

Проверка статуса (по файлу состояния, если демон запущен):

```bash
smartroute status
```

Остальные команды:

```bash
smartroute --help
smartroute run --help
smartroute status --help
smartroute tunnel --help
smartroute explain --help
smartroute events --help
smartroute dump --help
smartroute logs --help
smartroute metrics --help
smartroute sysopt --help
smartroute game --help
```

---

## Разработка

- Сборка: `make build`
- Тесты: `make test`
- Интеграционные тесты (требуют окружение): `make test-integration`
- Бенчмарки: `make bench`
- Очистка артефактов: `make clean`

Версия Go зафиксирована в `go.mod` (1.21) и в файле `.go-version` для goenv/asdf.

---

## Локальная разработка и тесты на Windows

Демон и сетевые адаптеры (ip, wg, nft, tc, conntrack) рассчитаны на **Linux**. На Windows можно собирать проект и гонять юнит-тесты.

### Что можно делать на Windows

1. **Установить Go** (1.21+): https://go.dev/dl/
2. **Клонировать и собрать:**
   ```powershell
   git clone https://github.com/bslie/smartroute.git
   cd smartroute
   go build -o smartroute.exe ./cmd/smartroute
   ```
3. **Проверить CLI без демона:**
   ```powershell
   .\smartroute.exe --help
   .\smartroute.exe run --help
   ```
4. **Запустить тесты** (логика domain, decision, store, engine и т.д. не зависит от ОС):
   ```powershell
   go test ./internal/...
   ```
   Если с `-race` будут проблемы — без флага:
   ```powershell
   go test ./internal/domain/... ./internal/decision/... ./internal/store/... ./internal/eventbus/... ./internal/memlog/... ./internal/engine/... ./internal/probe/... ./internal/observer/... ./internal/adapter/... -count=1
   ```

Команда `smartroute run` на Windows не имеет смысла: конфиг и адаптеры ожидают Linux-среду (маршруты, WireGuard, nftables и т.д.).

### Полноценное тестирование (демон + сеть)

Нужна Linux-среда:

- **WSL2:** установить Go в WSL, клонировать репозиторий в WSL и там выполнять `make build`, `make test`, `smartroute run` (при наличии WireGuard и прав).
- **Docker:** собрать образ с Go и Linux, запускать сборку и тесты в контейнере (`docker build -t smartroute-ci .` в корне репозитория).

---

## Быстрая отладка (ярусы A → B → C)

1. **Ярус A (после каждого изменения кода):** `go test ./internal/...` (в WSL удобно `make test`). Без root и без демона.
2. **Ярус B:** в WSL `make build` — проверка сборки Linux-бинарника.
3. **Ярус C (реальная сеть):** скопировать бинарник на тестовый хост (VPS/железо), `smartroute run`, затем `smartroute status`, `smartroute metrics`, `smartroute logs`, `smartroute logs probes`. Для деталей: `smartroute dump`, `smartroute explain <ip|домен>`.  
   **WSL:** демон можно запускать в WSL2, но для приёмочной проверки предпочтительнее VPS/железо. Браузер на Windows не открывает `127.0.0.1` процесса внутри WSL без проброса порта.

---

## Web UI

В конфиге задайте `web_ui_listen`, например `127.0.0.1:8899`. После запуска демона откройте в браузере **на той же машине** `http://127.0.0.1:8899/` — графики по истории метрик и таблица destinations. JSON API: `GET /api/v1/status` (снимок + история). Смена адреса требует перезапуска демона.

---

## KPI и сравнение с «голым» WireGuard

Честное **A/B на одном хосте:** остановить `smartroute`, сохранить сопоставимую схему WG/маршрутов (`wg-quick` и т.д.), измерить `ping` / `iperf3` к фиксированным целям; затем включить SmartRoute и повторить те же измерения.  
Ориентиры в рантайме: `smartroute metrics`, `smartroute logs probes`, счётчики в Web UI. Скрипт-напоминание: `scripts/kpi-ping.sh <хост>`.

---

## Лаборатория (2–3 машины)

Типичная схема: **машина 1** — хост с выходом в интернет, **SmartRoute** и **несколько интерфейсов WireGuard к VPS**; приложение выбирает туннель по политике. **Машины 2–3** — удалённые VPS-пиры, клиенты за шлюзом или внешние цели для проб. Docker Compose на одном хосте **не заменяет** такую топологию для реальных задержек и маршрутов, но подходит для сборки и тестов.
