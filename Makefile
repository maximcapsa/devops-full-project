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

.PHONY: sqlc
sqlc: ## Generate type-safe DB code for every service (sqlc)
	@for cfg in services/*/sqlc.yaml; do \
		[ -f "$$cfg" ] || continue; \
		dir=$$(dirname "$$cfg"); echo "sqlc generate ($$dir)"; \
		( cd "$$dir" && sqlc generate ) || exit 1; \
	done

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

# ── Cloud (Terraform / Helm) ─────────────────────────────────────────────────
TF_DEV := terraform -chdir=infra/envs/dev

.PHONY: infra-plan
infra-plan: ## Terraform plan for envs/dev (needs dev.tfvars + bootstrapped backend)
	$(TF_DEV) plan -var-file=dev.tfvars

.PHONY: infra-apply
infra-apply: ## Terraform apply for envs/dev — CREATES BILLABLE RESOURCES
	$(TF_DEV) apply -var-file=dev.tfvars

HELM ?= helm

.PHONY: charts
charts: ## Re-stamp shared templates into every service chart
	bash deploy/stamp-charts.sh

.PHONY: helm-lint
helm-lint: ## helm dependency update + lint + template the umbrella chart
	$(HELM) dependency update deploy/charts/ecommerce
	$(HELM) lint deploy/charts/ecommerce
	$(HELM) template ecommerce deploy/charts/ecommerce > /dev/null
	@echo "helm lint + template OK"

.PHONY: deploy
deploy: ## Deploy the umbrella chart to the current kubectl context
	$(HELM) dependency update deploy/charts/ecommerce
	$(HELM) upgrade --install ecommerce deploy/charts/ecommerce \
		--namespace ecommerce --create-namespace --wait --timeout 10m

.PHONY: destroy
destroy: ## Tear down ALL cloud infrastructure (terraform destroy)
	$(TF_DEV) destroy -var-file=dev.tfvars

# ── CI aggregate ─────────────────────────────────────────────────────────────
.PHONY: ci
ci: proto-lint test ## Run the checks CI runs locally
	@echo "CI checks passed"
