#!/bin/bash
# SmartRoute — скрипт установки из Git или локальной копии
# Получение репозитория (опционально), проверка зависимостей, компиляция

set -e

REPO_URL="${SMARTROUTE_REPO:-https://github.com/smartroute/smartroute.git}"
INSTALL_DIR="${SMARTROUTE_INSTALL_DIR:-/usr/local}"
BUILD_DIR="${SMARTROUTE_BUILD_DIR:-/tmp/smartroute-build}"
# Ветка по умолчанию должна совпадать с HEAD в репозитории (master/main)
VERSION="${SMARTROUTE_VERSION:-master}"
# Если задан SOURCE_DIR — собираем из этой папки (без git clone)
SOURCE_DIR="${SMARTROUTE_SOURCE_DIR:-}"

echo "[SmartRoute] Install"
echo "  INSTALL_DIR: $INSTALL_DIR"

# 1. Проверка Go
check_go() {
    if ! command -v go &>/dev/null; then
        echo "[ERROR] Go не найден. Установите Go 1.21+ (https://go.dev/dl/)."
        exit 1
    fi
    GO_VER=$(go version | awk '{print $3}' | sed 's/go//')
    MAJOR=$(echo "$GO_VER" | cut -d. -f1)
    MINOR=$(echo "$GO_VER" | cut -d. -f2)
    if [ "$MAJOR" -lt 1 ] || { [ "$MAJOR" -eq 1 ] && [ "$MINOR" -lt 21 ]; }; then
        echo "[ERROR] Требуется Go 1.21 или новее. Сейчас: $GO_VER"
        exit 1
    fi
    echo "[OK] Go $GO_VER"
}

# 2. Клонирование или использование локальной папки
prepare_src() {
    if [ -n "$SOURCE_DIR" ] && [ -d "$SOURCE_DIR" ]; then
        echo "[*] Сборка из локальной папки: $SOURCE_DIR"
        BUILD_DIR="$SOURCE_DIR"
        return
    fi
    if [ -d "$BUILD_DIR/.git" ]; then
        echo "[*] Обновление $BUILD_DIR..."
        (cd "$BUILD_DIR" && git fetch origin && git checkout "$VERSION" && git pull --ff-only origin "$VERSION" 2>/dev/null || true)
    else
        echo "[*] Клонирование $REPO_URL (branch $VERSION)..."
        rm -rf "$BUILD_DIR"
        git clone --depth 1 --branch "$VERSION" "$REPO_URL" "$BUILD_DIR" 2>/dev/null || \
        git clone --depth 1 "$REPO_URL" "$BUILD_DIR" && (cd "$BUILD_DIR" && git checkout "$VERSION" 2>/dev/null || true)
    fi
    echo "[OK] Исходники в $BUILD_DIR"
}

# 3. Загрузка зависимостей
deps() {
    echo "[*] Загрузка зависимостей Go..."
    (cd "$BUILD_DIR" && go mod download && go mod verify)
    echo "[OK] Зависимости готовы"
}

# 4. Сборка
build() {
    echo "[*] Сборка smartroute..."
    (cd "$BUILD_DIR" && CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}" -o smartroute ./cmd/smartroute)
    if [ ! -f "$BUILD_DIR/smartroute" ]; then
        echo "[ERROR] Сборка не создала бинарник."
        exit 1
    fi
    echo "[OK] Бинарник: $BUILD_DIR/smartroute"
}

# 5. Установка (опционально)
do_install() {
    if [ "$SKIP_INSTALL" = "1" ]; then
        echo "[*] SKIP_INSTALL=1 — установка в $INSTALL_DIR пропущена."
        echo "    Бинарник: $BUILD_DIR/smartroute"
        return
    fi
    echo "[*] Установка в $INSTALL_DIR/bin..."
    mkdir -p "$INSTALL_DIR/bin"
    install -m 755 "$BUILD_DIR/smartroute" "$INSTALL_DIR/bin/smartroute" 2>/dev/null || sudo install -m 755 "$BUILD_DIR/smartroute" "$INSTALL_DIR/bin/smartroute"
    echo "[OK] Установлено: $INSTALL_DIR/bin/smartroute"
    if [ -d "$BUILD_DIR/configs" ]; then
        mkdir -p "$INSTALL_DIR/share/smartroute"
        install -m 644 "$BUILD_DIR/configs/smartroute.example.yaml" "$INSTALL_DIR/share/smartroute/config.example.yaml" 2>/dev/null || sudo install -m 644 "$BUILD_DIR/configs/smartroute.example.yaml" "$INSTALL_DIR/share/smartroute/config.example.yaml" 2>/dev/null || true
        echo "[OK] Пример конфига: $INSTALL_DIR/share/smartroute/config.example.yaml"
    fi
}

# 6. Проверка после установки
verify() {
    if command -v smartroute &>/dev/null || [ -x "$INSTALL_DIR/bin/smartroute" ]; then
        EXEC=smartroute
        [ -x "$INSTALL_DIR/bin/smartroute" ] && EXEC="$INSTALL_DIR/bin/smartroute"
        "$EXEC" --help &>/dev/null && echo "[OK] smartroute --help выполнен успешно." || true
    fi
}

# Запуск шагов
check_go
prepare_src
deps
build
do_install
verify

echo ""
echo "Готово. Запуск: smartroute run -c /etc/smartroute/config.yaml"
echo "Или: $INSTALL_DIR/bin/smartroute run -c /path/to/config.yaml"
