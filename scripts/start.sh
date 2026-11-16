#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")/.."
sh scripts/setup.sh
docker compose up --build -d
echo 'NEXCODE: use APP_URL from .env (default http://localhost:3000).'
