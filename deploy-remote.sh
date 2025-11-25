#!/usr/bin/env bash
set -euo pipefail

# Run this on the VPS after rsync to finish deployment.
# Usage: ./deploy-remote.sh

ROOT_DIR="/home/antonio/notepub"

cd "${ROOT_DIR}"

if [ -f .env ]; then
  echo ">> Loading .env"
  set -a
  . ./.env
  set +a
fi

echo ">> Installing dependencies"
npm install

echo ">> Running Prisma migrate + generate"
npx prisma migrate deploy --schema packages/db/prisma/schema.prisma
npx prisma generate --schema packages/db/prisma/schema.prisma

echo ">> Building dashboard"
npm run build --workspace @notepub/dashboard

echo ">> Building renderer"
npm run build --workspace @notepub/renderer

echo ">> Restarting pm2 apps"
if pm2 describe notepub-dashboard >/dev/null 2>&1; then
  pm2 reload notepub-dashboard
else
  pm2 start npm --name notepub-dashboard -- run start --workspace @notepub/dashboard -- --hostname 127.0.0.1 --port 3100
fi

if pm2 describe notepub-renderer >/dev/null 2>&1; then
  pm2 reload notepub-renderer
else
  pm2 start npm --name notepub-renderer -- run start --workspace @notepub/renderer -- --hostname 127.0.0.1 --port 3200
fi

echo ">> Done."
