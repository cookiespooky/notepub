#!/usr/bin/env bash
set -euo pipefail

# Usage: ./deploy-dashboard.sh [user@host] [/remote/path]
# Defaults: antonio@your-server /home/antonio/notepub

HOST="${1:-antonio@your-server}"
REMOTE_DIR="${2:-/home/antonio/notepub}"

EXCLUDES=(
  --exclude ".git"
  --exclude "node_modules"
  --exclude ".next"
  --exclude ".turbo"
)

echo ">> Rsync code to ${HOST}:${REMOTE_DIR}"
rsync -avz --delete "${EXCLUDES[@]}" ./ "${HOST}:${REMOTE_DIR}/"

ENV_FILES=(
  ".env"
  "apps/dashboard/.env.local"
)

echo ">> Sync env files"
for f in "${ENV_FILES[@]}"; do
  if [ -f "$f" ]; then
    rsync -avz "$f" "${HOST}:${REMOTE_DIR}/${f}"
  fi
done

echo ">> Done. Code and env files synced. Run build/migrate/restart manually on the VPS."
