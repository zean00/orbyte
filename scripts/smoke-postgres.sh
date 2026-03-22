#!/usr/bin/env bash
set -euo pipefail

APP_ADDRESS="${APP_ADDRESS:-127.0.0.1:18080}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

APP_ENV=development \
APP_AUTH_DEV_MODE=true \
APP_ADDRESS="$APP_ADDRESS" \
APP_JWT_SECRET="${APP_JWT_SECRET:-dev-secret}" \
APP_BOOTSTRAP_ADMIN_PASSWORD="${APP_BOOTSTRAP_ADMIN_PASSWORD:-admin123!}" \
go run ./cmd/server >/tmp/orbyte-smoke.log 2>&1 &
SERVER_PID=$!

cleanup() {
  kill "$SERVER_PID" >/dev/null 2>&1 || true
  wait "$SERVER_PID" >/dev/null 2>&1 || true
}
trap cleanup EXIT

for _ in $(seq 1 30); do
  if curl --silent --fail "http://${APP_ADDRESS}/readyz" >/dev/null; then
    exit 0
  fi
  sleep 1
done

echo "postgres smoke test failed; server log:" >&2
cat /tmp/orbyte-smoke.log >&2
exit 1
