#!/bin/bash
# SmartRoute — скрипт установки из Git или локальной копии
# Проверка и установка зависимостей, получение репозитория (опционально), компиляция

set -e

REPO_URL="${SMARTROUTE_REPO:-https://github.com/bslie/smartroute.git}"
INSTALL_DIR="${SMARTROUTE_INSTALL_DIR:-/usr/local}"
BUILD_DIR="${SMARTROUTE_BUILD_DIR:-/tmp/smartroute-build}"
# Ветка по умолчанию должна совпадать с HEAD в репозитории (master/main)
VERSION="${SMARTROUTE_VERSION:-master}"
# Если задан SOURCE_DIR — собираем из этой папки (без git clone)
SOURCE_DIR="${SMARTROUTE_SOURCE_DIR:-}"

echo "[SmartRoute] Install"
echo "  INSTALL_DIR: $INSTALL_DIR"

# Определение менеджера пакетов
detect_pkg_manager() {
    if command -v apt-get &>/dev/null; then
        PKG_MGR="apt"
        PKG_UPDATE="apt-get update -qq"
        PKG_INSTALL="apt-get install -y -qq"
    elif command -v dnf &>/dev/null; then
        PKG_MGR="dnf"
        PKG_UPDATE="true"
        PKG_INSTALL="dnf install -y -q"
    elif command -v yum &>/dev/null; then
        PKG_MGR="yum"
        PKG_UPDATE="yum check-update -q || true"
        PKG_INSTALL="yum install -y -q"
    elif command -v apk &>/dev/null; then
        PKG_MGR="apk"
        PKG_UPDATE="apk update"
        PKG_INSTALL="apk add --no-cache"
    elif command -v brew &>/dev/null; then
        PKG_MGR="brew"
        PKG_UPDATE="true"
        PKG_INSTALL="brew install"
    else
        PKG_MGR=""
    fi
}

# Установка через пакетный менеджер (требует sudo при необходимости)
run_pkg_install() {
    local need_sudo=""
    [ "$(id -u)" -ne 0 ] && need_sudo="sudo"
    $need_sudo $PKG_UPDATE 2>/dev/null || true
    $need_sudo $PKG_INSTALL "$@" 2>/dev/null || $need_sudo $PKG_INSTALL "$@"
}

# 1. Проверка и установка Git (нужен при клонировании)
check_git() {
    if [ -n "$SOURCE_DIR" ] && [ -d "$SOURCE_DIR" ]; then
        return
    fi
    if command -v git &>/dev/null; then
        echo "[OK] Git $(git --version | awk '{print $3}')"
        return
    fi
    echo "[*] Git не найден, устанавливаю..."
    detect_pkg_manager
    case "$PKG_MGR" in
        apt) run_pkg_install git ;;
        dnf|yum) run_pkg_install git ;;
        apk) run_pkg_install git ;;
        brew) run_pkg_install git ;;
        *) echo "[ERROR] Git не найден. Установите git вручную."; exit 1 ;;
    esac
    if ! command -v git &>/dev/null; then
        echo "[ERROR] Не удалось установить Git."
        exit 1
    fi
    echo "[OK] Git установлен"
}

# 2. Проверка и установка Go (1.21+)
check_go() {
    local need_install=0
    if ! command -v go &>/dev/null; then
        need_install=1
    else
        GO_VER=$(go version | awk '{print $3}' | sed 's/go//')
        MAJOR=$(echo "$GO_VER" | cut -d. -f1)
        MINOR=$(echo "$GO_VER" | cut -d. -f2)
        if [ "$MAJOR" -lt 1 ] || { [ "$MAJOR" -eq 1 ] && [ "$MINOR" -lt 21 ]; }; then
            echo "[*] Go $GO_VER устарел, требуется 1.21+. Пробую обновить..."
            need_install=1
        else
            echo "[OK] Go $GO_VER"
            return
        fi
    fi

    if [ "$need_install" -eq 1 ]; then
        [ -z "$PKG_MGR" ] && detect_pkg_manager
        echo "[*] Устанавливаю Go..."
        case "$PKG_MGR" in
            apt) run_pkg_install golang-go ;;
            dnf|yum) run_pkg_install golang ;;
            apk) run_pkg_install go ;;
            brew) run_pkg_install go ;;
            *) true ;;
        esac
    fi

    if ! command -v go &>/dev/null; then
        echo "[ERROR] Go не найден. Установите Go 1.21+ вручную: https://go.dev/dl/"
        exit 1
    fi
    GO_VER=$(go version | awk '{print $3}' | sed 's/go//')
    MAJOR=$(echo "$GO_VER" | cut -d. -f1)
    MINOR=$(echo "$GO_VER" | cut -d. -f2)
    if [ "$MAJOR" -lt 1 ] || { [ "$MAJOR" -eq 1 ] && [ "$MINOR" -lt 21 ]; }; then
        echo "[ERROR] Требуется Go 1.21 или новее. Сейчас: $GO_VER. Установите вручную: https://go.dev/dl/"
        exit 1
    fi
    echo "[OK] Go $GO_VER"
}

# Проверка, что каталог — корень репозитория smartroute (.git + go.mod)
is_repo_root() {
    local dir="$1"
    [ -d "${dir}/.git" ] && [ -f "${dir}/go.mod" ]
}

# 3. Клонирование или использование локальной папки
prepare_src() {
    # Явно указана папка с исходниками
    if [ -n "$SOURCE_DIR" ] && [ -d "$SOURCE_DIR" ]; then
        echo "[*] Сборка из папки: $SOURCE_DIR"
        BUILD_DIR="$SOURCE_DIR"
        echo "[OK] Исходники в $BUILD_DIR"
        return
    fi
    # Запуск из корня репозитория: по расположению скрипта, PWD и (при sudo) по дому пользователя
    _script="${BASH_SOURCE[0]}"
    [[ "$_script" != /* ]] && _script="${PWD}/${_script}"
    _script_dir="$(cd "$(dirname "$_script")" && pwd)"
    REPO_ROOT="$(dirname "$_script_dir")"
    if is_repo_root "$REPO_ROOT"; then
        echo "[*] Сборка из репозитория (скрипт в дереве): $REPO_ROOT"
        BUILD_DIR="$REPO_ROOT"
        echo "[OK] Исходники в $BUILD_DIR"
        return
    fi
    if is_repo_root "$PWD"; then
        echo "[*] Сборка из текущей папки: $PWD"
        BUILD_DIR="$PWD"
        echo "[OK] Исходники в $BUILD_DIR"
        return
    fi
    if [ -n "${SUDO_USER:-}" ]; then
        _user_home="$(getent passwd "$SUDO_USER" 2>/dev/null | cut -d: -f6)"
        if [ -n "$_user_home" ]; then
            for _cand in "$_user_home/smartroute" "$_user_home/smartroute/smartroute"; do
                if is_repo_root "$_cand"; then
                    echo "[*] Сборка из репозитория пользователя $SUDO_USER: $_cand"
                    BUILD_DIR="$_cand"
                    echo "[OK] Исходники в $BUILD_DIR"
                    return
                fi
            done
        fi
    fi
    if [ -d "$BUILD_DIR/.git" ] && is_repo_root "$BUILD_DIR"; then
        echo "[*] Обновление $BUILD_DIR..."
        (cd "$BUILD_DIR" && git fetch origin && git checkout "$VERSION" && git pull --ff-only origin "$VERSION" 2>/dev/null || true)
    else
        if [ -d "$BUILD_DIR" ]; then
            echo "[*] Каталог $BUILD_DIR неполный или устарел — переклонирую."
            rm -rf "$BUILD_DIR"
        fi
        echo "[*] Клонирование $REPO_URL (branch $VERSION)..."
        git clone --depth 1 --branch "$VERSION" "$REPO_URL" "$BUILD_DIR" 2>/dev/null || \
        git clone --depth 1 "$REPO_URL" "$BUILD_DIR" && (cd "$BUILD_DIR" && git checkout "$VERSION" 2>/dev/null || true)
    fi
    echo "[OK] Исходники в $BUILD_DIR"
}

# 4. Загрузка зависимостей
deps() {
    echo "[*] Загрузка зависимостей Go..."
    (cd "$BUILD_DIR" && go mod download && go mod verify)
    echo "[OK] Зависимости готовы"
}

# 5. Сборка
build() {
    echo "[*] Сборка smartroute..."
    (cd "$BUILD_DIR" && CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}" -o smartroute ./cmd/smartroute)
    if [ ! -f "$BUILD_DIR/smartroute" ]; then
        echo "[ERROR] Сборка не создала бинарник."
        exit 1
    fi
    echo "[OK] Бинарник: $BUILD_DIR/smartroute"
}

# 6. Установка (опционально)
do_install() {
    if [ "$SKIP_INSTALL" = "1" ]; then
        echo "[*] SKIP_INSTALL=1 — установка в $INSTALL_DIR пропущена."
        echo "    Бинарник: $BUILD_DIR/smartroute"
        return
    fi
    echo "[*] Установка в $INSTALL_DIR/bin..."
    mkdir -p "$INSTALL_DIR/bin" 2>/dev/null || sudo mkdir -p "$INSTALL_DIR/bin"
    install -m 755 "$BUILD_DIR/smartroute" "$INSTALL_DIR/bin/smartroute" 2>/dev/null || sudo install -m 755 "$BUILD_DIR/smartroute" "$INSTALL_DIR/bin/smartroute"
    echo "[OK] Установлено: $INSTALL_DIR/bin/smartroute"
    if [ -d "$BUILD_DIR/configs" ]; then
        mkdir -p "$INSTALL_DIR/share/smartroute" 2>/dev/null || sudo mkdir -p "$INSTALL_DIR/share/smartroute"
        install -m 644 "$BUILD_DIR/configs/smartroute.example.yaml" "$INSTALL_DIR/share/smartroute/config.example.yaml" 2>/dev/null || sudo install -m 644 "$BUILD_DIR/configs/smartroute.example.yaml" "$INSTALL_DIR/share/smartroute/config.example.yaml" 2>/dev/null || true
        echo "[OK] Пример конфига: $INSTALL_DIR/share/smartroute/config.example.yaml"
    fi
}

# 7. Проверка после установки
verify() {
    if command -v smartroute &>/dev/null || [ -x "$INSTALL_DIR/bin/smartroute" ]; then
        EXEC=smartroute
        [ -x "$INSTALL_DIR/bin/smartroute" ] && EXEC="$INSTALL_DIR/bin/smartroute"
        "$EXEC" --help &>/dev/null && echo "[OK] smartroute --help выполнен успешно." || true
    fi
}

# Запуск шагов
detect_pkg_manager
check_git
check_go
prepare_src
deps
build
do_install
verify

echo ""
echo "Готово. Запуск: smartroute run -c /etc/smartroute/config.yaml"
echo "Или: $INSTALL_DIR/bin/smartroute run -c /path/to/config.yaml"
