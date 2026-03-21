# SmartRoute — сборка и тесты (целевая ОС: Linux)
#
# Сборка в WSL (Ubuntu): cd /mnt/w/projects/smartroute && make build
# Или одной командой с Windows: wsl -d Ubuntu-24.04 make -C /mnt/w/projects/smartroute build

VERSION ?= 0.1.0
BINARY  := smartroute
LDFLAGS := -s -w -X main.version=$(VERSION)

# Использовать локальный Go из .go/bin, если системный go недоступен
ifneq (,$(wildcard .go/bin/go))
  export PATH := $(CURDIR)/.go/bin:$(PATH)
  export GOPATH := $(CURDIR)/.gopath
  export GOMODCACHE := $(CURDIR)/.gopath/pkg/mod
endif
GO ?= go

.PHONY: build build-linux test test-integration bench clean install

# Обычная сборка (на Linux/WSL — нативный бинарник под Linux)
build:
	CGO_ENABLED=0 $(GO) build -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/smartroute

# Явная сборка под Linux (удобно при запуске make из Windows через WSL)
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/smartroute

build-upx: build
	upx --best $(BINARY) 2>/dev/null || true

test:
	$(GO) test ./internal/domain/... ./internal/decision/... ./internal/store/... ./internal/eventbus/... ./internal/memlog/... ./internal/engine/... ./internal/observer/... ./internal/probe/... ./internal/adapter/... -race -count=1

# Историческое имя: отдельных integration-тестов с тегом пока нет — прогоняются те же пакеты adapter.
test-integration:
	$(GO) test ./internal/adapter/... -tags=integration -race -count=1 2>/dev/null || $(GO) test ./internal/adapter/... -count=1

bench:
	$(GO) test ./internal/observer/... ./internal/decision/... ./internal/store/... -bench=. -benchmem

clean:
	rm -f $(BINARY)

install: build
	install -d $(DESTDIR)/usr/local/bin
	install -m 755 $(BINARY) $(DESTDIR)/usr/local/bin/
	install -d $(DESTDIR)/etc/smartroute
	install -m 644 configs/smartroute.example.yaml $(DESTDIR)/etc/smartroute/config.example.yaml
