#!/bin/sh
# Простой замер задержки к цели (для сравнения до/после SmartRoute на одном хосте).
# Использование: ./scripts/kpi-ping.sh 1.1.1.1
set -e
HOST="${1:-1.1.1.1}"
echo "Ping to $HOST (5 packets)"
ping -c 5 "$HOST" || true
