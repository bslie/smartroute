#!/bin/bash
# SmartRoute — удаление файлов, установленных install.sh
# Удаляет бинарник и пример конфига из INSTALL_DIR. Опционально — каталог /etc/smartroute.

set -e

INSTALL_DIR="${SMARTROUTE_INSTALL_DIR:-/usr/local}"
PURGE_CONFIG=0

for arg in "$@"; do
    case "$arg" in
        --purge-config) PURGE_CONFIG=1 ;;
        -h|--help)
            echo "Usage: $0 [--purge-config]"
            echo "  Удаляет $INSTALL_DIR/bin/smartroute и $INSTALL_DIR/share/smartroute/"
            echo "  --purge-config  также удалить /etc/smartroute (ваш конфиг и пример)"
            exit 0
            ;;
    esac
done

run_rm() {
    local path="$1"
    if [ ! -e "$path" ]; then
        return
    fi
    rm -f "$path" 2>/dev/null || sudo rm -f "$path"
    echo "[OK] Удалён: $path"
}

run_rmdir() {
    local path="$1"
    [ ! -d "$path" ] && return
    [ -n "$(ls -A "$path" 2>/dev/null)" ] && return
    rmdir "$path" 2>/dev/null || sudo rmdir "$path" 2>/dev/null || true
    echo "[OK] Удалён каталог: $path"
}

echo "[SmartRoute] Uninstall"
echo "  INSTALL_DIR: $INSTALL_DIR"

run_rm "$INSTALL_DIR/bin/smartroute"
run_rm "$INSTALL_DIR/share/smartroute/config.example.yaml"
run_rm "$INSTALL_DIR/share/smartroute/install-wireguard.sh"
run_rmdir "$INSTALL_DIR/share/smartroute"

if [ "$PURGE_CONFIG" -eq 1 ]; then
    echo "[*] --purge-config: удаляю /etc/smartroute..."
    if [ -d /etc/smartroute ]; then
        sudo rm -rf /etc/smartroute
        echo "[OK] Удалён: /etc/smartroute"
    fi
fi

echo ""
echo "Готово. SmartRoute удалён из $INSTALL_DIR."
