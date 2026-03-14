# SmartRoute — сборка и тесты

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

.PHONY: build test test-integration bench clean install

build:
	CGO_ENABLED=0 $(GO) build -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/smartroute

build-upx: build
	upx --best $(BINARY) 2>/dev/null || true

test:
	$(GO) test ./internal/domain/... ./internal/decision/... ./internal/store/... ./internal/eventbus/... ./internal/memlog/... ./internal/engine/... ./internal/observer/... ./internal/probe/... ./internal/adapter/... -race -count=1

test-integration:
	$(GO) test ./internal/adapter/... -tags=integration -race -count=1 2>/dev/null || $(GO) test ./internal/adapter/... -count=1

bench:
	$(GO) test ./internal/observer/... ./internal/decision/... -bench=. -benchmem

clean:
	rm -f $(BINARY)

install: build
	install -d $(DESTDIR)/usr/local/bin
	install -m 755 $(BINARY) $(DESTDIR)/usr/local/bin/
	install -d $(DESTDIR)/etc/smartroute
	install -m 644 configs/smartroute.example.yaml $(DESTDIR)/etc/smartroute/config.example.yaml
