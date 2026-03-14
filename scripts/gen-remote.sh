#!/bin/bash
# SmartRoute gen-remote — bootstrap WireGuard на удалённом VPS (exit node).
# Supported: Debian 11+, Ubuntu 20.04+
# Идемпотентно: check -> apply. Повторный запуск безопасен.

set -e

REMOTE="${1:-}"
WG_NAME="${2:-wg0}"
LISTEN_PORT="${3:-51820}"

if [ -z "$REMOTE" ]; then
  echo "Usage: $0 <user@host> [wg_interface_name] [listen_port]"
  echo "  Example: $0 root://vps.example.com wg0 51820"
  exit 1
fi

# Парсим user@host (поддержка root@ или только host)
REMOTE_USER="${REMOTE%%@*}"
REMOTE_HOST="${REMOTE##*@}"
if [ "$REMOTE_USER" = "$REMOTE_HOST" ]; then
  REMOTE_USER="root"
fi

SSH_TARGET="${REMOTE_USER}@${REMOTE_HOST}"

run_remote() {
  ssh "$SSH_TARGET" "$@"
}

# 1) Проверка: уже есть интерфейс?
run_remote "ip link show $WG_NAME 2>/dev/null" && {
  echo "[*] Interface $WG_NAME already exists, showing public key"
  run_remote "wg show $WG_NAME public-key 2>/dev/null" || true
  exit 0
}

# 2) Установка wireguard-tools если нет
run_remote "which wg 2>/dev/null" || {
  echo "[*] Installing wireguard-tools..."
  run_remote "apt-get update -qq && apt-get install -y -qq wireguard-tools"
}

# 3) Генерация ключей на удалённой стороне
run_remote "mkdir -p /etc/wireguard"
run_remote "test -f /etc/wireguard/${WG_NAME}_private.key" || \
  run_remote "wg genkey | tee /etc/wireguard/${WG_NAME}_private.key | wg pubkey > /etc/wireguard/${WG_NAME}_public.key"
run_remote "chmod 600 /etc/wireguard/${WG_NAME}_private.key"

# 4) Создание интерфейса (идемпотентно)
run_remote "ip link add $WG_NAME type wireguard 2>/dev/null" || true
run_remote "wg set $WG_NAME private-key /etc/wireguard/${WG_NAME}_private.key"
run_remote "ip link set $WG_NAME up"

# 5) NAT (iptables) — идемпотентно
run_remote "iptables -t nat -C POSTROUTING -o eth0 -j MASQUERADE 2>/dev/null" || \
  run_remote "iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE"

# 6) Вывод public key
echo "[OK] Public key for $WG_NAME:"
run_remote "wg show $WG_NAME public-key"
