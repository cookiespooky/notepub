#!/usr/bin/env bash
set -euo pipefail

# Usage: ./deploy.sh [--staging|--prod] [user@host] [/remote/path]
# Defaults:
#   prod:    antonio@109.73.207.243:/home/antonio/notepub
#   staging: antonio@109.73.207.243:/home/antonio/notepub_staging

ENVIRONMENT="prod"
POSITIONAL=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --staging)
      ENVIRONMENT="staging"
      shift
      ;;
    --prod)
      ENVIRONMENT="prod"
      shift
      ;;
    *)
      POSITIONAL+=("$1")
      shift
      ;;
  esac
done

set -- "${POSITIONAL[@]}"

if [[ "${ENVIRONMENT}" == "staging" ]]; then
  DEFAULT_HOST="antonio@109.73.207.243"
  DEFAULT_REMOTE_DIR="/home/antonio/notepub_staging"
  ENV_FILES=(".env.staging")
else
  DEFAULT_HOST="antonio@109.73.207.243"
  DEFAULT_REMOTE_DIR="/home/antonio/notepub"
  ENV_FILES=(".env")
fi

HOST="${1:-$DEFAULT_HOST}"
REMOTE_DIR="${2:-$DEFAULT_REMOTE_DIR}"

EXCLUDES=(
  --exclude ".git"
  --exclude "node_modules"
  --exclude ".next"
  --exclude ".turbo"
  --exclude ".env.local"
  --exclude "apps/dashboard/.env.local"
  --exclude "apps/renderer/.env.local"
  --exclude "apps/*/.env.local"
  --include "deploy-remote.sh"
)

echo ">> Rsync code to ${HOST}:${REMOTE_DIR}"
rsync -avz --delete "${EXCLUDES[@]}" ./ "${HOST}:${REMOTE_DIR}/"

echo ">> Sync env files for ${ENVIRONMENT}"
for f in "${ENV_FILES[@]}"; do
  if [ -f "$f" ]; then
    rsync -avz "$f" "${HOST}:${REMOTE_DIR}/${f}"
  fi
done

echo ">> Done. Code and env files synced. Run build/migrate/restart manually on the VPS."
