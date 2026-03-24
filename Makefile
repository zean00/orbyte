APP_ADDRESS ?= :8080
APP_JWT_SECRET ?= dev-secret
APP_BOOTSTRAP_ADMIN_PASSWORD ?= admin123!
DATABASE_URL ?= postgres://orbyte:orbyte@127.0.0.1:5432/orbyte?sslmode=disable

.PHONY: test lint coverage contracts ui-build migrate-up migrate-status run run-postgres smoke-postgres docs-build docs-serve

test:
	./scripts/test.sh

lint:
	./scripts/lint.sh

coverage:
	./scripts/coverage.sh

contracts:
	./scripts/verify-contracts.sh

ui-build:
	npm run build:ui

docs-build:
	. .venv-docs/bin/activate && mkdocs build --strict

docs-serve:
	. .venv-docs/bin/activate && mkdocs serve

migrate-up:
	APP_JWT_SECRET=$(APP_JWT_SECRET) DATABASE_URL=$(DATABASE_URL) go run ./cmd/migrate up

migrate-status:
	DATABASE_URL=$(DATABASE_URL) go run ./cmd/migrate status

run:
	APP_ADDRESS=$(APP_ADDRESS) APP_AUTH_DEV_MODE=true APP_JWT_SECRET=$(APP_JWT_SECRET) APP_BOOTSTRAP_ADMIN_PASSWORD=$(APP_BOOTSTRAP_ADMIN_PASSWORD) go run ./cmd/server

run-postgres:
	APP_ADDRESS=$(APP_ADDRESS) APP_ENV=development APP_JWT_SECRET=$(APP_JWT_SECRET) APP_BOOTSTRAP_ADMIN_PASSWORD=$(APP_BOOTSTRAP_ADMIN_PASSWORD) DATABASE_URL=$(DATABASE_URL) go run ./cmd/server

smoke-postgres:
	APP_ADDRESS=127.0.0.1:18080 APP_ENV=development APP_JWT_SECRET=$(APP_JWT_SECRET) APP_BOOTSTRAP_ADMIN_PASSWORD=$(APP_BOOTSTRAP_ADMIN_PASSWORD) DATABASE_URL=$(DATABASE_URL) ./scripts/smoke-postgres.sh
