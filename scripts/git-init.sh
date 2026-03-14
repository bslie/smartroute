#!/bin/bash
# Инициализация git-репозитория SmartRoute (запускать из корня проекта или из scripts/)

set -e

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

if ! command -v git &>/dev/null; then
  echo "Установите git: apt-get install git"
  exit 1
fi

needs_commit=false
if [ ! -d .git ]; then
  git init
  needs_commit=true
else
  if [ -z "$(git rev-parse -q --verify HEAD 2>/dev/null)" ]; then
    needs_commit=true
  else
    echo "Репозиторий уже инициализирован и есть коммиты."
    git status --short
    exit 0
  fi
fi

# Локальная идентичность для коммитов, если глобально не задана
if [ -z "$(git config user.email 2>/dev/null)" ]; then
  git config user.email "smartroute@local"
  echo "Задан локальный user.email: smartroute@local"
fi
if [ -z "$(git config user.name 2>/dev/null)" ]; then
  git config user.name "SmartRoute"
  echo "Задан локальный user.name: SmartRoute"
fi

git add -A
git status

if [ "$needs_commit" = true ]; then
  echo ""
  echo "Первый коммит:"
  git commit -m "Initial commit: SmartRoute — операционная сетевая система на Go"
fi
echo ""
echo "Готово. Репозиторий в $REPO_ROOT"
