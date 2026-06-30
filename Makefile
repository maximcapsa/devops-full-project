# ── Event-Driven E-Commerce Platform ────────────────────────────────────────
# Run `make help` for a list of targets. Local dev needs: go, docker, buf.
# buf is installed via `make tools` into $(go env GOPATH)/bin — ensure that is on PATH.

SHELL := bash
.DEFAULT_GOAL := help

GOPATH_BIN := $(shell go env GOPATH)/bin
export PATH := $(GOPATH_BIN):$(PATH)

BUF            ?= buf
GOLANGCI_LINT  ?= golangci-lint
COMPOSE        ?= docker compose
SERVICES       := bff product order inventory payment notification

# ── Meta ─────────────────────────────────────────────────────────────────────
.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| sort | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

.PHONY: tools
tools: ## Install dev tooling (buf, golangci-lint, sqlc, migrate)
	go install github.com/bufbuild/buf/cmd/buf@v1.50.0
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.61.0
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.1

# ── Proto ────────────────────────────────────────────────────────────────────
.PHONY: proto
proto: ## Generate Go from protobufs (buf generate)
	$(BUF) generate

.PHONY: proto-lint
proto-lint: ## Lint protobufs
	$(BUF) lint

.PHONY: proto-breaking
proto-breaking: ## Check protos for breaking changes vs main
	$(BUF) breaking --against '.git#branch=main'

# ── Go ───────────────────────────────────────────────────────────────────────
.PHONY: build
build: ## Build all Go services into ./bin
	@for s in $(SERVICES); do \
		if [ -f services/$$s/main.go ]; then \
			echo "building $$s"; go build -o bin/$$s ./services/$$s || exit 1; \
		fi; \
	done

.PHONY: test
test: ## Run Go unit tests
	go test ./...

.PHONY: lint
lint: ## Run golangci-lint
	$(GOLANGCI_LINT) run

.PHONY: tidy
tidy: ## go mod tidy
	go mod tidy

# ── Local stack (docker-compose) ─────────────────────────────────────────────
.PHONY: up
up: ## Start the full local stack (Postgres, Kafka, services)
	$(COMPOSE) up --build -d

.PHONY: down
down: ## Stop the local stack
	$(COMPOSE) down

.PHONY: logs
logs: ## Tail local stack logs
	$(COMPOSE) logs -f

.PHONY: ps
ps: ## Show local stack status
	$(COMPOSE) ps

# ── Cloud (Terraform / Helm) — implemented in later phases ───────────────────
.PHONY: deploy
deploy: ## Deploy to the k3s cluster via Helm (Phase 8+)
	@echo "deploy: implemented in Phase 8/9"

.PHONY: destroy
destroy: ## Tear down ALL cloud infrastructure (terraform destroy)
	@echo "destroy: implemented in Phase 7/10"

# ── CI aggregate ─────────────────────────────────────────────────────────────
.PHONY: ci
ci: proto-lint test ## Run the checks CI runs locally
	@echo "CI checks passed"
