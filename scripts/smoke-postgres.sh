#!/usr/bin/env bash
set -euo pipefail

APP_ADDRESS="${APP_ADDRESS:-127.0.0.1:18080}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_PORT="${APP_ADDRESS##*:}"
cd "$ROOT_DIR"

stop_listener() {
  local pid
  pid="$(lsof -tiTCP:${APP_PORT} -sTCP:LISTEN 2>/dev/null | head -n 1 || true)"
  if [[ -n "${pid}" ]]; then
    kill "${pid}" >/dev/null 2>&1 || true
    for _ in $(seq 1 10); do
      if ! kill -0 "${pid}" >/dev/null 2>&1; then
        return
      fi
      sleep 1
    done
    kill -9 "${pid}" >/dev/null 2>&1 || true
  fi
}

stop_listener

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
  stop_listener
}
trap cleanup EXIT

for _ in $(seq 1 30); do
  if grep -q "orbyte server listening on ${APP_ADDRESS}" /tmp/orbyte-smoke.log 2>/dev/null; then
    if lsof -tiTCP:${APP_PORT} -sTCP:LISTEN >/dev/null 2>&1; then
      exit 0
    fi
  fi
  if ! kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

echo "postgres smoke test failed; server log:" >&2
cat /tmp/orbyte-smoke.log >&2
exit 1
