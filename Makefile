APP_ADDRESS ?= 127.0.0.1:18110
APP_BASE_URL ?= http://$(APP_ADDRESS)
APP_JWT_SECRET ?= dev-secret
APP_BOOTSTRAP_ADMIN_PASSWORD ?= admin123!
DATABASE_URL ?= postgres://orbyte:orbyte@127.0.0.1:5432/orbyte?sslmode=disable

RUN_DIR ?= .run
APP_PID_FILE ?= $(RUN_DIR)/orbyte-postgres.pid
APP_LOG_FILE ?= $(RUN_DIR)/orbyte-postgres.log

AGENT_SCENARIO ?= inventory_replenishment_execute
AGENT_SEED_OUTPUT ?= /tmp/agentproof-inventory-replenishment.json
POS_SEED_OUTPUT ?= /tmp/orbyte-pos-seed.json

.PHONY: test lint coverage contracts frontend-build frontend-verify ui-build migrate-up migrate-status run run-postgres smoke-postgres docs-build docs-serve
.PHONY: app-start-postgres app-stop-postgres app-status-postgres app-restart-postgres app-wait-postgres
.PHONY: db-reset-postgres seed-agent-continuity seed-pos seed-all

test:
	./scripts/test.sh

frontend-build:
	cd frontend && npm install && npm run build

frontend-verify:
	cd frontend && npm ci && npm run check:generated && npm run typecheck && npm run build

ui-build:
	npm run build:ui

lint:
	./scripts/lint.sh

coverage:
	./scripts/coverage.sh

contracts:
	./scripts/verify-contracts.sh

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

app-start-postgres: migrate-up
	@mkdir -p $(RUN_DIR)
	@if [ -f "$(APP_PID_FILE)" ] && kill -0 "$$(cat "$(APP_PID_FILE)")" 2>/dev/null; then \
		echo "App already running with PID $$(cat "$(APP_PID_FILE)") at $(APP_BASE_URL)"; \
	else \
		if [ -f "$(APP_PID_FILE)" ]; then rm -f "$(APP_PID_FILE)"; fi; \
		echo "Starting app on $(APP_BASE_URL)"; \
		nohup env APP_ADDRESS=$(APP_ADDRESS) APP_ENV=development APP_JWT_SECRET=$(APP_JWT_SECRET) APP_BOOTSTRAP_ADMIN_PASSWORD=$(APP_BOOTSTRAP_ADMIN_PASSWORD) DATABASE_URL=$(DATABASE_URL) go run ./cmd/server >"$(APP_LOG_FILE)" 2>&1 & echo $$! >"$(APP_PID_FILE)"; \
	fi
	@$(MAKE) app-wait-postgres

app-wait-postgres:
	@attempts=0; \
	until curl -fsS "$(APP_BASE_URL)/healthz" >/dev/null 2>&1; do \
		attempts=$$((attempts+1)); \
		if [ $$attempts -ge 60 ]; then \
			echo "App did not become ready. Recent log output:"; \
			if [ -f "$(APP_LOG_FILE)" ]; then tail -n 40 "$(APP_LOG_FILE)"; fi; \
			exit 1; \
		fi; \
		sleep 1; \
	done; \
	echo "App is ready at $(APP_BASE_URL)"

app-stop-postgres:
	@if [ ! -f "$(APP_PID_FILE)" ]; then \
		echo "App is not running"; \
		exit 0; \
	fi
	@pid="$$(cat "$(APP_PID_FILE)")"; \
	if kill -0 "$$pid" 2>/dev/null; then \
		echo "Stopping app PID $$pid"; \
		kill "$$pid"; \
		for _ in 1 2 3 4 5 6 7 8 9 10; do \
			if ! kill -0 "$$pid" 2>/dev/null; then break; fi; \
			sleep 1; \
		done; \
		if kill -0 "$$pid" 2>/dev/null; then \
			echo "Force stopping app PID $$pid"; \
			kill -9 "$$pid"; \
		fi; \
	else \
		echo "Removing stale pid file for PID $$pid"; \
	fi; \
	rm -f "$(APP_PID_FILE)"

app-status-postgres:
	@if [ -f "$(APP_PID_FILE)" ] && kill -0 "$$(cat "$(APP_PID_FILE)")" 2>/dev/null; then \
		echo "App running with PID $$(cat "$(APP_PID_FILE)") at $(APP_BASE_URL)"; \
		curl -fsS "$(APP_BASE_URL)/healthz" && echo; \
	else \
		echo "App is not running"; \
		exit 1; \
	fi

app-restart-postgres: app-stop-postgres app-start-postgres

db-reset-postgres: app-stop-postgres
	@echo "Resetting PostgreSQL schema for $(DATABASE_URL)"
	@psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -c "DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public;"
	@$(MAKE) migrate-up

seed-agent-continuity: app-start-postgres
	go run ./cmd/agentproof seed \
		--base-url "$(APP_BASE_URL)" \
		--username admin \
		--password "$(APP_BOOTSTRAP_ADMIN_PASSWORD)" \
		--scenario "$(AGENT_SCENARIO)" \
		--output "$(AGENT_SEED_OUTPUT)"

seed-pos:
	DATABASE_URL="$(DATABASE_URL)" POS_SEED=1 go test -run TestSeedPOSTerminalSyntheticScenario -v ./internal/platform/app
	@echo "POS seed manifest is written by the test harness to $(POS_SEED_OUTPUT)"

seed-all: seed-agent-continuity seed-pos
	@echo "Continuity seed manifest: $(AGENT_SEED_OUTPUT)"
	@echo "POS seed manifest: $(POS_SEED_OUTPUT)"
