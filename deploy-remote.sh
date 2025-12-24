#!/usr/bin/env bash
set -euo pipefail

# Run this on the VPS after rsync to finish deployment.
# Usage: ./deploy-remote.sh [--staging|--prod] [root_dir_override]

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
  ROOT_DIR_DEFAULT="/home/antonio/notepub_staging"
  ENV_FILE=".env.staging"
  DEFAULT_DASHBOARD_PORT=3300
  DEFAULT_RENDERER_PORT=3400
  DEFAULT_SYNC_PORT=3401
else
  ROOT_DIR_DEFAULT="/home/antonio/notepub"
  ENV_FILE=".env"
  DEFAULT_DASHBOARD_PORT=3100
  DEFAULT_RENDERER_PORT=3200
  DEFAULT_SYNC_PORT=3201
fi

ROOT_DIR="${1:-$ROOT_DIR_DEFAULT}"

cd "${ROOT_DIR}"

if [ -f "${ENV_FILE}" ]; then
  echo ">> Loading ${ENV_FILE}"
  set -a
  . "./${ENV_FILE}"
  set +a
fi

if [[ "${ENVIRONMENT}" == "staging" ]]; then
  PM2_SUFFIX="-staging"
else
  PM2_SUFFIX=""
fi

pm2_app_name() {
  local base="$1"
  echo "${base}${PM2_SUFFIX}"
}

DASHBOARD_PORT="${DASHBOARD_PORT:-$DEFAULT_DASHBOARD_PORT}"
RENDERER_PORT="${RENDERER_PORT:-$DEFAULT_RENDERER_PORT}"
SYNC_PORT="${SYNC_PORT:-$DEFAULT_SYNC_PORT}"

echo ">> Installing dependencies"
npm install

echo ">> Running Prisma migrate + generate"
npx prisma migrate deploy --schema packages/db/prisma/schema.prisma
npx prisma generate --schema packages/db/prisma/schema.prisma

echo ">> Building dashboard"
npm run build --workspace @notepub/dashboard

echo ">> Building renderer"
npm run build --workspace @notepub/renderer

echo ">> Building sync service"
npm run build --workspace @notepub/sync

echo ">> Restarting pm2 apps"
if pm2 describe "$(pm2_app_name notepub-dashboard)" >/dev/null 2>&1; then
  # Reload with updated environment variables
  pm2 reload "$(pm2_app_name notepub-dashboard)" --update-env
else
  pm2 start npm --name "$(pm2_app_name notepub-dashboard)" -- run start --workspace @notepub/dashboard -- --hostname 127.0.0.1 --port "${DASHBOARD_PORT}"
fi

if pm2 describe "$(pm2_app_name notepub-renderer)" >/dev/null 2>&1; then
  pm2 reload "$(pm2_app_name notepub-renderer)" --update-env
else
  pm2 start npm --name "$(pm2_app_name notepub-renderer)" -- run start --workspace @notepub/renderer -- --hostname 127.0.0.1 --port "${RENDERER_PORT}"
fi

if pm2 describe "$(pm2_app_name notepub-sync)" >/dev/null 2>&1; then
  pm2 reload "$(pm2_app_name notepub-sync)" --update-env
else
  pm2 start npm --name "$(pm2_app_name notepub-sync)" -- run start --workspace @notepub/sync
fi

echo ">> Done."
