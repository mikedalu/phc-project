# ══════════════════════════════════════════════════════════════
#  PHC Records — developer commands
#
#  Run `make help` for the list.
#
#  Migrations read MIGRATION_DATABASE_URL by sourcing ../.env. Note the
#  URLs in .env MUST be double-quoted: they contain "&", and an unquoted
#  ampersand makes the shell background the rest of the line, leaving the
#  variable empty. See .env.example.
# ══════════════════════════════════════════════════════════════

SHELL := /bin/bash
.DEFAULT_GOAL := help

# Go build/module caches. Overridable, because $HOME can be read-only in
# containers, CI sandboxes and restricted accounts — where the default
# (~/.cache/go-build) fails with a confusing "read-only file system" error that
# looks like a code problem. Re-run as:  make test-api GOCACHE=/tmp/gocache
GOCACHE     ?= $(shell go env GOCACHE 2>/dev/null)
GOMODCACHE  ?= $(shell go env GOMODCACHE 2>/dev/null)
export GOCACHE
export GOMODCACHE

API_DIR    := api
WEB_DIR    := web
MIGRATIONS := $(API_DIR)/db/migrations
SEEDS      := $(API_DIR)/db/seed

# Loads .env, then runs goose against the DIRECT (non-pooled) endpoint.
# The subshell keeps the exported variables out of the rest of make.
define goose
	@test -f .env || { echo "!! .env missing. Run: cp .env.example .env"; exit 1; }
	@set -a; . ./.env; set +a; \
	 test -n "$$MIGRATION_DATABASE_URL" || { \
	   echo "!! MIGRATION_DATABASE_URL is empty. Are the URLs double-quoted in .env?"; exit 1; }; \
	 cd $(API_DIR) && goose -dir db/migrations postgres "$$MIGRATION_DATABASE_URL" $(1)
endef

.PHONY: help
help: ## Show this help
	@echo "PHC Records — available targets:"
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# ─── Database ──────────────────────────────────────────────────

.PHONY: migrate-up
migrate-up: ## Apply all pending migrations (uses the DIRECT Neon URL)
	$(call goose,up)

prune: ## Trim the replication change log to each table's retention
	cd api && go run ./cmd/prune

.PHONY: migrate-down
migrate-down: ## Roll back exactly one migration
	$(call goose,down)

.PHONY: migrate-status
migrate-status: ## Show which migrations are applied
	$(call goose,status)

.PHONY: migrate-version
migrate-version: ## Print the current migration version
	$(call goose,version)

.PHONY: migrate-verify
migrate-verify: ## Prove reversibility: up -> down -> up on the latest migration
	@echo ">> The round trip. If this fails, the migration is wrong."
	$(call goose,up)
	$(call goose,down)
	$(call goose,up)
	@echo ">> OK: up -> down -> up left the schema where it started."

.PHONY: migrate-validate
migrate-validate: ## Check migration files without connecting to a database
	cd $(API_DIR) && goose -dir db/migrations validate

.PHONY: psql
psql: ## Open psql against the DIRECT Neon endpoint
	@set -a; . ./.env; set +a; psql "$$MIGRATION_DATABASE_URL"

.PHONY: seed
seed: ## Load development seed data (idempotent)
	@test -f $(SEEDS)/dev_seed.sql || { echo "!! $(SEEDS)/dev_seed.sql not found"; exit 1; }
	@set -a; . ./.env; set +a; \
	 psql "$$MIGRATION_DATABASE_URL" -v ON_ERROR_STOP=1 -f $(SEEDS)/dev_seed.sql

# ─── API ───────────────────────────────────────────────────────

.PHONY: api
api: ## Run the API locally with hot-ish reload (go run)
	@test -f .env || { echo "!! .env missing. Run: cp .env.example .env"; exit 1; }
	@set -a; . ./.env; set +a; \
	 test -n "$$DATABASE_URL" || { \
	   echo "!! DATABASE_URL is empty. Are the URLs double-quoted in .env?"; exit 1; }; \
	 cd $(API_DIR) && go run ./cmd/api

.PHONY: build
build: ## Build the API binary into api/bin/
	cd $(API_DIR) && go build -o bin/api ./cmd/api

.PHONY: tidy
tidy: ## Tidy Go module dependencies
	cd $(API_DIR) && go mod tidy

.PHONY: fmt
fmt: ## Format all Go source
	cd $(API_DIR) && gofmt -w .

.PHONY: vet
vet: ## Run go vet
	cd $(API_DIR) && go vet ./...

.PHONY: test-api
test-api: ## Run Go unit tests (no database; integration tests skip)
	cd $(API_DIR) && go test ./... -count=1

.PHONY: test-api-db
test-api-db: ## Run ALL Go tests including integration ones, against Neon
	@test -f .env || { echo "!! .env missing"; exit 1; }
	@set -a; . ./.env; set +a; \
	 test -n "$$MIGRATION_DATABASE_URL" || { echo "!! MIGRATION_DATABASE_URL empty"; exit 1; }; \
	 cd $(API_DIR) && SERVICE_TEST_DATABASE_URL="$$MIGRATION_DATABASE_URL" go test ./... -count=1 -v

.PHONY: test-api-cover
test-api-cover: ## Go tests with a coverage summary
	cd $(API_DIR) && go test ./... -count=1 -cover

.PHONY: check
check: fmt vet test-api ## Format, vet and test the backend

# ─── Web ───────────────────────────────────────────────────────

.PHONY: web
web: ## Run the frontend dev server
	cd $(WEB_DIR) && pnpm dev

.PHONY: web-build
web-build: ## Production build of the frontend
	cd $(WEB_DIR) && pnpm build

.PHONY: web-test
web-test: ## Run frontend unit tests
	cd $(WEB_DIR) && pnpm test

.PHONY: install
install: ## Install frontend dependencies
	cd $(WEB_DIR) && pnpm install

# ─── Local smoke test ──────────────────────────────────────────

.PHONY: smoke
smoke: ## Curl the running API's health endpoints
	@echo ">> /health"
	@curl -sS -m 20 localhost:8080/health | (command -v jq >/dev/null && jq . || cat)
	@echo ">> /ready"
	@curl -sS -m 20 localhost:8080/ready | (command -v jq >/dev/null && jq . || cat)

.PHONY: smoke-meta
smoke-meta: ## Curl /api/v1/meta (proves the full DB round trip)
	@curl -sS -m 20 localhost:8080/api/v1/meta | (command -v jq >/dev/null && jq . || cat)

# ─── DANGER ────────────────────────────────────────────────────

.PHONY: db-reset
db-reset: ## DESTRUCTIVE: drop everything and re-migrate. Blocks if APP_ENV=production
	@if grep -qE '^APP_ENV=production' .env 2>/dev/null; then \
		echo "!! REFUSING: APP_ENV=production in .env"; exit 1; fi
	@read -p "This DESTROYS all data in MIGRATION_DATABASE_URL. Type 'yes' to continue: " ok; \
		[ "$$ok" = "yes" ] || { echo "aborted"; exit 1; }
	$(call goose,reset)
	$(call goose,up)
	@echo ">> Database reset complete."
