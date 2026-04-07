APP_ADDRESS ?= 127.0.0.1:18110
APP_BASE_URL ?= http://$(APP_ADDRESS)
APP_JWT_SECRET ?= dev-secret
APP_BOOTSTRAP_ADMIN_PASSWORD ?= admin123!
POSTGRES_HOST_PORT ?= 55432
DATABASE_URL ?= postgres://orbyte:orbyte@127.0.0.1:$(POSTGRES_HOST_PORT)/orbyte?sslmode=disable

RUN_DIR ?= .run
APP_PID_FILE ?= $(RUN_DIR)/orbyte-postgres.pid
APP_LOG_FILE ?= $(RUN_DIR)/orbyte-postgres.log

AGENT_SCENARIO ?= inventory_replenishment_execute
AGENT_SEED_OUTPUT ?= /tmp/agentproof-inventory-replenishment.json
POS_SEED_OUTPUT ?= /tmp/orbyte-pos-seed.json
DASHBOARD_SEED_OUTPUT ?= /tmp/orbyte-dashboard-seed.json
SHOWCASE_SCENARIO ?= retail_recovery_showcase
SHOWCASE_SEED_OUTPUT ?= /tmp/orbyte-showcase-retail-recovery.json

.PHONY: test lint coverage contracts frontend-build frontend-verify ui-build migrate-up migrate-status run run-postgres smoke-postgres docs-build docs-serve
.PHONY: app-start-postgres app-stop-postgres app-status-postgres app-restart-postgres app-wait-postgres
.PHONY: db-reset-postgres seed-agent-continuity seed-pos seed-dashboard seed-showcase-demo seed-all reset-and-seed demo-continuity demo-showcase

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
		echo "App pidfile not found"; \
	else \
		pid="$$(cat "$(APP_PID_FILE)")"; \
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
		rm -f "$(APP_PID_FILE)"; \
	fi
	@listener_pid="$$(lsof -tiTCP:$$(printf '%s\n' "$(APP_ADDRESS)" | awk -F: '{print $$NF}') -sTCP:LISTEN 2>/dev/null | head -n 1)"; \
	if [ -n "$$listener_pid" ]; then \
		echo "Stopping listener PID $$listener_pid on $(APP_ADDRESS)"; \
		kill "$$listener_pid" 2>/dev/null || true; \
		for _ in 1 2 3 4 5 6 7 8 9 10; do \
			if ! kill -0 "$$listener_pid" 2>/dev/null; then break; fi; \
			sleep 1; \
		done; \
		if kill -0 "$$listener_pid" 2>/dev/null; then \
			echo "Force stopping listener PID $$listener_pid"; \
			kill -9 "$$listener_pid" 2>/dev/null || true; \
		fi; \
	else \
		echo "No listener running on $(APP_ADDRESS)"; \
	fi

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
	DATABASE_URL="$(DATABASE_URL)" POS_SEED=1 go test -count=1 -run TestSeedPOSTerminalSyntheticScenario -v ./internal/platform/app
	@echo "POS seed manifest is written by the test harness to $(POS_SEED_OUTPUT)"

seed-dashboard:
	DATABASE_URL="$(DATABASE_URL)" DASHBOARD_SEED=1 go test -count=1 -run TestSeedDashboardSyntheticScenario -v ./internal/platform/app
	@echo "Dashboard seed manifest is written by the test harness to $(DASHBOARD_SEED_OUTPUT)"

seed-showcase-demo: app-start-postgres
	go run ./cmd/agentproof seed \
		--base-url "$(APP_BASE_URL)" \
		--username admin \
		--password "$(APP_BOOTSTRAP_ADMIN_PASSWORD)" \
		--scenario "$(SHOWCASE_SCENARIO)" \
		--output "$(SHOWCASE_SEED_OUTPUT)"
	@echo "Showcase seed manifest: $(SHOWCASE_SEED_OUTPUT)"

seed-all: seed-agent-continuity seed-pos seed-dashboard
	@echo "Continuity seed manifest: $(AGENT_SEED_OUTPUT)"
	@echo "POS seed manifest: $(POS_SEED_OUTPUT)"
	@echo "Dashboard seed manifest: $(DASHBOARD_SEED_OUTPUT)"

reset-and-seed: db-reset-postgres app-start-postgres seed-all

demo-continuity: seed-agent-continuity
	@echo "Continuity manifest: $(AGENT_SEED_OUTPUT)"
	@python3 -c 'import json; path="$(AGENT_SEED_OUTPUT)"; manifest=json.load(open(path, "r", encoding="utf-8")); warehouse=manifest.get("entities", {}).get("warehouse", {}).get("code", ""); vendor=manifest.get("entities", {}).get("vendor", {}).get("name", ""); print("Warehouse:", warehouse); print("Vendor:", vendor) if vendor else None; print(); [print("{}. {}".format(idx, text)) for idx, text in ((idx, prompt.get("prompt", "").strip()) for idx, prompt in enumerate(manifest.get("prompt_pack", []), start=1)) if text]'

demo-showcase: seed-showcase-demo
	@echo "Showcase manifest: $(SHOWCASE_SEED_OUTPUT)"
	@python3 -c 'import json; path="$(SHOWCASE_SEED_OUTPUT)"; manifest=json.load(open(path, "r", encoding="utf-8")); base=manifest.get("base_url", "").rstrip("/"); routes=manifest.get("routes", {}); print("POS:", base + routes.get("pos_terminal", "")); print("Dashboard:", base + routes.get("dashboard", "")); print("Agent:", base + routes.get("agent", "")); print(); print("Questions to ask the agent:"); [print("{}. {}".format(idx, text)) for idx, text in ((idx, prompt.get("prompt", "").strip()) for idx, prompt in enumerate(manifest.get("prompt_pack", []), start=1)) if text]'
