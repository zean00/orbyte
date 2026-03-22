#!/usr/bin/env bash
set -euo pipefail

export APP_ENV="${APP_ENV:-development}"
export APP_ADDRESS="${APP_ADDRESS:-:8080}"
export APP_JWT_SECRET="${APP_JWT_SECRET:-dev-secret}"
export APP_BOOTSTRAP_ADMIN_PASSWORD="${APP_BOOTSTRAP_ADMIN_PASSWORD:-admin123!}"
export DATABASE_URL="${DATABASE_URL:-postgres://orbyte:orbyte@127.0.0.1:5432/orbyte?sslmode=disable}"

go run ./cmd/migrate up
go run ./cmd/server
