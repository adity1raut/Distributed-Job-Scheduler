# ---------------------------------------------------------------------------
# Distributed Job Scheduler — development and deployment tasks.
#
#   make help     list every target
#   make up       start the full stack locally
#   make smoke    prove a running deployment actually executes jobs
# ---------------------------------------------------------------------------

SHELL := /bin/bash
.DEFAULT_GOAL := help

# Explicit -f lists so the automatic docker-compose.override.yml is applied
# for dev and suppressed for prod — the difference decides whether the
# database port is published, so it must never be left implicit.
COMPOSE      := docker compose
COMPOSE_DEV  := $(COMPOSE) -f docker-compose.yml -f docker-compose.override.yml
COMPOSE_PROD := $(COMPOSE) -f docker-compose.yml -f docker-compose.prod.yml

REGISTRY  ?= ghcr.io
NAMESPACE ?= adity1raut/distributed-job-scheduler
# Tag from git: the exact tag on a release commit, else <branch>-<sha>. Never
# "latest" for a deploy — you cannot roll back to a tag that moves.
TAG       ?= $(shell git describe --tags --exact-match 2>/dev/null || echo "$$(git rev-parse --abbrev-ref HEAD | tr '/' '-')-$$(git rev-parse --short HEAD)")

API_IMAGE := $(REGISTRY)/$(NAMESPACE)/api:$(TAG)
WEB_IMAGE := $(REGISTRY)/$(NAMESPACE)/web:$(TAG)

# Integration tests are skipped unless these point at a real Postgres/Redis.
TEST_DATABASE_URL ?= postgres://postgres:postgres@localhost:5433/jobscheduler?sslmode=disable
TEST_REDIS_ADDR   ?= localhost:6380

##@ Help

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} \
		/^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

##@ Setup

.PHONY: init
init: .env ## Create .env from the template if it does not exist

.env:
	@cp .env.example .env
	@echo "Created .env from .env.example."
	@echo "For anything but local use, replace JWT_SECRET and POSTGRES_PASSWORD:"
	@echo "  openssl rand -base64 48"

.PHONY: secrets
secrets: ## Print freshly generated production secrets
	@echo "JWT_SECRET=$$(openssl rand -base64 48 | tr -d '\n')"
	@echo "POSTGRES_PASSWORD=$$(openssl rand -base64 24 | tr -d '\n')"
	@echo "REDIS_PASSWORD=$$(openssl rand -base64 24 | tr -d '\n')"

##@ Local development

.PHONY: up
up: init ## Build and start the full stack (detached)
	$(COMPOSE_DEV) up -d --build
	@$(MAKE) --no-print-directory status

.PHONY: down
down: ## Stop the stack, keeping data volumes
	$(COMPOSE_DEV) down

.PHONY: clean
clean: ## Stop the stack and DELETE all data volumes
	$(COMPOSE_DEV) down -v --remove-orphans

.PHONY: restart
restart: down up ## Recreate the stack from scratch

.PHONY: logs
logs: ## Tail logs from every service (make logs SVC=worker for one)
	$(COMPOSE_DEV) logs -f --tail=100 $(SVC)

.PHONY: status
status: ## Show container status and the dashboard URL
	@$(COMPOSE_DEV) ps --format 'table {{.Service}}\t{{.Status}}\t{{.Ports}}'
	@echo ""
	@echo "Dashboard: http://localhost:$$(grep -E '^WEB_PORT=' .env 2>/dev/null | cut -d= -f2 || echo 3000)"
	@echo "Sign in with the BOOTSTRAP_EMAIL / BOOTSTRAP_PASSWORD from .env"

.PHONY: ps
ps: status ## Alias for status

.PHONY: shell-api
shell-api: ## Open a shell in the api container
	$(COMPOSE_DEV) exec api /bin/sh

.PHONY: psql
psql: ## Open psql against the stack's database
	$(COMPOSE_DEV) exec postgres psql -U $${POSTGRES_USER:-postgres} -d $${POSTGRES_DB:-jobscheduler}

.PHONY: redis-cli
redis-cli: ## Open redis-cli against the stack's cache
	$(COMPOSE_DEV) exec redis redis-cli

.PHONY: scale-workers
scale-workers: ## Scale the worker fleet (make scale-workers N=5)
	$(COMPOSE_DEV) up -d --scale worker=$(or $(N),3) --no-recreate worker

##@ Database

.PHONY: migrate-up
migrate-up: ## Apply all pending migrations
	$(COMPOSE_DEV) run --rm migrate

.PHONY: migrate-down
migrate-down: ## Roll back the most recent migration
	$(COMPOSE_DEV) run --rm --entrypoint migrate migrate \
		-path=/migrations -database="$$(grep -E '^DATABASE_URL=' .env | cut -d= -f2- || echo)" down 1

.PHONY: migrate-status
migrate-status: ## Show the current schema version
	$(COMPOSE_DEV) exec postgres psql -U $${POSTGRES_USER:-postgres} -d $${POSTGRES_DB:-jobscheduler} \
		-c 'SELECT version, dirty FROM schema_migrations;'

##@ Quality

.PHONY: fmt
fmt: ## Format Go sources
	cd server && gofmt -w .

.PHONY: lint
lint: lint-go lint-web ## Run every linter

.PHONY: lint-go
lint-go: ## Vet and format-check the Go code
	@cd server && if [ -n "$$(gofmt -l .)" ]; then \
		echo "gofmt found unformatted files:"; gofmt -l .; exit 1; \
	fi
	cd server && go vet ./...

.PHONY: lint-web
lint-web: ## Lint the frontend
	cd ui-interface && npm ci --prefer-offline --no-audit && npm run lint

.PHONY: test
test: ## Run unit tests (integration tests skip without a database)
	cd server && go test ./... -race -count=1

.PHONY: test-integration
test-integration: ## Run the full suite against the running stack's database
	cd server && TEST_DATABASE_URL="$(TEST_DATABASE_URL)" TEST_REDIS_ADDR="$(TEST_REDIS_ADDR)" \
		go test ./... -race -count=1 -v

.PHONY: smoke
smoke: ## End-to-end check: submit a job and prove a worker runs it
	./deploy/scripts/smoke-test.sh http://localhost:$$(grep -E '^WEB_PORT=' .env 2>/dev/null | cut -d= -f2 || echo 3000)

##@ Images

.PHONY: build
build: ## Build both images with the resolved git tag
	docker build --build-arg VERSION=$(TAG) -t $(API_IMAGE) ./server
	docker build --build-arg VERSION=$(TAG) -t $(WEB_IMAGE) ./ui-interface
	@echo "Built $(API_IMAGE)"
	@echo "Built $(WEB_IMAGE)"

.PHONY: push
push: build ## Push both images to the registry
	docker push $(API_IMAGE)
	docker push $(WEB_IMAGE)

.PHONY: scan
scan: build ## Scan both images for known CVEs
	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
		aquasec/trivy:latest image --severity HIGH,CRITICAL --exit-code 1 $(API_IMAGE)
	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
		aquasec/trivy:latest image --severity HIGH,CRITICAL --exit-code 1 $(WEB_IMAGE)

##@ Deployment

.PHONY: deploy
deploy: ## Start the production stack from registry images
	IMAGE_TAG=$(TAG) IMAGE_REGISTRY=$(REGISTRY) IMAGE_NAMESPACE=$(NAMESPACE) \
		$(COMPOSE_PROD) up -d --pull always
	@$(COMPOSE_PROD) ps

.PHONY: deploy-check
deploy-check: ## Render the production config without starting anything
	IMAGE_TAG=$(TAG) IMAGE_REGISTRY=$(REGISTRY) IMAGE_NAMESPACE=$(NAMESPACE) \
		$(COMPOSE_PROD) config
