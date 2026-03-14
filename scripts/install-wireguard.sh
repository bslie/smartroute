#!/bin/sh
# SmartRoute — установка WireGuard на машине, где запускается smartroute run.
# Вызывается автоматически при smartroute run, если wg не найден.
# Идемпотентно: если wg уже есть, ничего не делаем.

set -e

if command -v wg >/dev/null 2>&1; then
	echo "[SmartRoute] WireGuard уже установлен: $(wg --version 2>/dev/null || echo 'wg')"
	exit 0
fi

# Определение менеджера пакетов
if command -v apt-get >/dev/null 2>&1; then
	need_sudo=""
	[ "$(id -u)" -ne 0 ] && need_sudo="sudo"
	$need_sudo apt-get update -qq
	$need_sudo apt-get install -y -qq wireguard wireguard-tools
elif command -v dnf >/dev/null 2>&1; then
	need_sudo=""
	[ "$(id -u)" -ne 0 ] && need_sudo="sudo"
	$need_sudo dnf install -y -q wireguard-tools 2>/dev/null || $need_sudo dnf install -y -q wireguard-tools
elif command -v yum >/dev/null 2>&1; then
	need_sudo=""
	[ "$(id -u)" -ne 0 ] && need_sudo="sudo"
	$need_sudo yum install -y -q wireguard-tools 2>/dev/null || true
elif command -v apk >/dev/null 2>&1; then
	need_sudo=""
	[ "$(id -u)" -ne 0 ] && need_sudo="sudo"
	$need_sudo apk add --no-cache wireguard-tools
else
	echo "[SmartRoute] Не удалось определить пакетный менеджер (apt/dnf/yum/apk). Установите WireGuard вручную." >&2
	exit 1
fi

if ! command -v wg >/dev/null 2>&1; then
	echo "[SmartRoute] Установка выполнена, но команда wg не найдена. Перезапустите smartroute run." >&2
	exit 1
fi

echo "[SmartRoute] WireGuard установлен: $(wg --version 2>/dev/null || echo 'wg')"
