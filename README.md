# Event-Driven E-Commerce Platform

A small but realistic event-driven e-commerce backend + storefront, built to demonstrate a
complete DevOps / microservices stack: synchronous **gRPC**, an asynchronous **Kafka** event
backbone, **Postgres**, **Terraform** IaC, a multi-node **k3s** cluster that survives node
failure, and **CI/CD** — all on **AWS Graviton (arm64)** and kept deliberately cheap.

> **Status:** under construction (built phase by phase — see [Build plan](#build-plan)).

## Architecture (cost-optimized variant)

```
Browser ──REST/JSON──> bff ──gRPC──> product / order / inventory / notification
                                         │
                          order ──produce──> Kafka (Redpanda) ──consume──> inventory
                                                                           payment
                                                                           notification
```

All services are Go. The `bff` exposes REST via grpc-gateway and speaks gRPC to internal
services. `order` produces `OrderPlaced`; `inventory`, `payment`, and `notification` are
Kafka consumers that react and emit follow-on events.

## Cost decisions (deliberate tradeoffs vs a production build)

This deployment targets **~$19–20/mo on-demand (~$0 on signup credits)**. To get there it
trades several "enterprise correct" choices for cheap ones — documented here on purpose:

| Decision | Cheap choice taken | Tradeoff |
|---|---|---|
| Kubernetes | self-managed **k3s** on EC2 | no EKS control-plane fee (~$73/mo saved) |
| Ingress | **Elastic IP + k3s Traefik** | no ALB (~$16/mo saved); single entry node |
| Kafka | **Redpanda single-node, in-cluster** | no MSK, no dedicated EC2; **replication factor 1** |
| Database | **Postgres in-cluster** on `local-path` volume | no RDS; no managed backups; data on one node |
| Cluster size | **1 server + 1 spot agent** (2 nodes) | smallest cluster that still demos node-failure HA |
| Networking | **public subnets + tight SGs** | no NAT Gateway (~$32/mo saved) |
| DB topology | one Postgres, **schema-per-service** | not database-per-service |
| CPU arch | **Graviton / arm64** (`t4g.*`, spot agents) | all images must be `linux/arm64` |

`make destroy` tears everything down and an AWS Budgets alarm guards against runaway cost.

## Repository layout

```
proto/      .proto files (buf) · gen/  generated Go (committed)
services/   bff product order inventory payment notification
pkg/        shared Go: config, log, postgres, kafka, health, otel
db/         migrations/ (golang-migrate) · queries/ (sqlc)
frontend/   Next.js storefront
deploy/     Helm charts (one per service + umbrella)
infra/      Terraform (network, compute, ecr, iam, state, budgets)
.github/    CI/CD workflows
```

## Quick start (local)

```bash
make tools          # one-time: install buf, golangci-lint, sqlc, migrate
cp .env.example .env
make up             # full stack via docker-compose
# storefront -> http://localhost:3000 , bff REST -> http://localhost:8080
make down
```

## Build plan

Phases 0–10, each verified before the next — see `claude-code-build-prompt.md` §12.
**Cloud infrastructure is built but never applied without explicit confirmation.**

## License

MIT (TBD).
