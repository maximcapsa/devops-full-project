# Build Brief — Event-Driven E-Commerce Platform (Go · gRPC · Kafka · k3s on AWS)

> **How to use this file:** Paste it into Claude Code as the project brief. Read the whole thing first, ask me any clarifying questions, then build **phase by phase** (see "Build plan"). After each phase, stop and show me what works before continuing. **Do not run `terraform apply` against real AWS, or create any billable cloud resource, without asking me first** — local development comes first and must be fully working before we touch the cloud.

---

## 1. What we're building

A small but realistic event-driven e-commerce backend plus a web storefront. It exists to demonstrate a complete DevOps/microservices stack: synchronous gRPC between services, an asynchronous Kafka event backbone, a relational database, infrastructure as code, a multi-node Kubernetes cluster that survives a node failure, and CI/CD.

**Services (all Go):**
- `bff` — backend-for-frontend / API gateway. Speaks REST/JSON to the browser, gRPC to internal services.
- `product` — product catalog. Owns products in Postgres. gRPC server.
- `order` — order placement. Owns orders in Postgres. gRPC server. Produces `OrderPlaced` events to Kafka.
- `inventory` — Kafka consumer of `OrderPlaced`. Adjusts stock in Postgres. gRPC server for stock queries.
- `payment` — Kafka consumer of `OrderPlaced`. Simulates payment, emits `PaymentCompleted` / `PaymentFailed`.
- `notification` — Kafka consumer of order/payment events. Records notifications; exposes them so the UI can show live order status.

**Frontend:** Next.js (App Router, TypeScript, Tailwind) storefront: browse products, place an order, watch order status update.

---

## 2. Hard constraints (do not violate)

- **Cloud:** AWS only. Keep it cheap — target well under ~$45/month, ~$0 while on signup credits.
- **NO** Amazon EKS (control-plane fee). Use **self-managed k3s on EC2**.
- **NO** Amazon MSK. Kafka runs on its **own dedicated EC2 instance** (KRaft mode, in a container).
- **NO** Multi-AZ RDS. Single-AZ `db.t4g.micro` (or `small`).
- **NO** NAT Gateway (≈$32/mo). Use public subnets with tight security groups, plus VPC gateway endpoints for S3/DynamoDB where useful.
- **Graviton/arm64 everywhere.** EC2 nodes are `t4g.*` (ARM). All container images **must** be `linux/arm64` (or multi-arch). Building amd64-only images is a common failure — they will not run on these nodes.
- **Language:** Go for every backend service and all tooling where reasonable. TypeScript only for the frontend.
- **IaC:** Terraform for all infrastructure. No click-ops.
- **Secrets:** never in git. Use env vars locally, Kubernetes Secrets in-cluster, GitHub OIDC for CI→AWS (no long-lived AWS keys).
- Always provide a working **`make destroy`** and a low **AWS Budgets alarm** so costs can't run away.

---

## 3. Pinned tech stack

| Concern | Choice |
|---|---|
| Language | Go (latest stable 1.x), arm64 target |
| RPC | gRPC via `google.golang.org/grpc`; protobuf managed with **buf** (`buf lint`, `buf generate`) |
| Browser-facing API | `bff` exposes REST/JSON using **grpc-gateway** generated from the same protos |
| DB driver / queries | `pgx/v5` + **sqlc** (type-safe queries) |
| Migrations | **golang-migrate** |
| Kafka client | **franz-go** (`github.com/twmb/franz-go`) |
| Config | 12-factor env vars (`caarlos0/env` or stdlib); no config files with secrets |
| Logging | stdlib `log/slog`, JSON handler |
| Tracing/metrics (later) | OpenTelemetry SDK; metrics to CloudWatch or `/metrics` |
| Frontend | Next.js (App Router) + TypeScript + Tailwind; data via `fetch` to `bff` REST |
| Kafka broker | Apache Kafka in **KRaft mode** (no ZooKeeper), run as a container on a dedicated EC2. Redpanda single-node is an acceptable drop-in alternative. |
| Orchestration | **k3s** (CNCF-conformant), multi-node |
| App deploy | **Helm** charts (one per service); k3s built-in **Traefik** ingress |
| Registry | Amazon ECR (arm64 images) |
| IaC | Terraform; remote state in S3 + DynamoDB lock table |
| CI/CD | GitHub Actions + AWS OIDC |
| Local dev | docker-compose (Postgres + Kafka + all services) |

---

## 4. Repository layout (monorepo)

```
.
├── proto/                     # .proto files + buf.yaml + buf.gen.yaml
│   ├── product/v1/...
│   ├── order/v1/...
│   ├── inventory/v1/...
│   └── events/v1/...          # Kafka event schemas (OrderPlaced, PaymentCompleted, ...)
├── gen/                       # generated Go (gRPC + grpc-gateway) — committed
├── services/
│   ├── bff/
│   ├── product/
│   ├── order/
│   ├── inventory/
│   ├── payment/
│   └── notification/
├── pkg/                       # shared Go: config, log, postgres, kafka, health, otel
├── db/
│   ├── migrations/            # golang-migrate SQL files (per service or per schema)
│   └── queries/               # sqlc input
├── frontend/                  # Next.js app
├── deploy/
│   └── charts/                # one Helm chart per service + an umbrella chart
├── infra/
│   ├── modules/
│   │   ├── network/           # VPC, public subnets (2+ AZ), SGs, VPC endpoints
│   │   ├── compute/           # k3s server + agent ASG, launch templates, user-data bootstrap
│   │   ├── kafka/             # dedicated EC2 + gp3 EBS + SG + Route53 record
│   │   ├── rds/               # single-AZ postgres
│   │   ├── alb/               # ALB/NLB → Traefik NodePort (or use NLB)
│   │   ├── ecr/               # one repo per service
│   │   └── iam/               # GitHub OIDC provider + deploy roles
│   ├── envs/
│   │   └── dev/               # tfvars, backend config
│   └── backend.tf
├── .github/workflows/         # ci.yml (lint/test/build), deploy.yml (push + helm)
├── docker-compose.yml         # full local stack
├── Makefile                   # proto, build, test, up, down, deploy, destroy
└── README.md
```

---

## 5. Service contracts

Define these in `proto/` and generate with buf. Sketches (flesh out as needed):

**product/v1/product.proto**
```proto
service ProductService {
  rpc ListProducts(ListProductsRequest) returns (ListProductsResponse);
  rpc GetProduct(GetProductRequest) returns (Product);
}
message Product { string id = 1; string name = 2; string description = 3;
  int64 price_cents = 4; int32 stock = 5; }
```

**order/v1/order.proto**
```proto
service OrderService {
  rpc PlaceOrder(PlaceOrderRequest) returns (Order);
  rpc GetOrder(GetOrderRequest) returns (Order);
}
message OrderItem { string product_id = 1; int32 quantity = 2; }
message Order { string id = 1; string status = 2; repeated OrderItem items = 3;
  int64 total_cents = 4; string created_at = 5; }
```

**inventory/v1/inventory.proto** — `CheckStock`, `GetStock`.

**events/v1/events.proto** — Kafka payloads, serialized as protobuf or JSON (pick one and be consistent):
- `OrderPlaced { order_id, items[], total_cents, occurred_at }`
- `PaymentCompleted { order_id, occurred_at }` / `PaymentFailed { order_id, reason }`
- `StockReserved { order_id }` / `StockRejected { order_id, reason }`

**Kafka topics:** `orders.placed`, `payments.completed`, `payments.failed`, `inventory.reserved`, `inventory.rejected`. Each consumer uses a distinct consumer group. Document partition/replication assumptions: **single broker ⇒ replication factor 1** (state this tradeoff in the README).

---

## 6. Data model

One RDS Postgres instance, **schema-per-service** (`product`, `order`, `inventory`, `notification`). Document explicitly in the README that this is a deliberate cost compromise vs database-per-service. Sketch:

- `product.products(id uuid pk, name, description, price_cents, created_at)`
- `inventory.stock(product_id uuid pk, available int, reserved int)`
- `order.orders(id uuid pk, status, total_cents, created_at)` + `order.order_items(order_id, product_id, quantity)`
- `notification.notifications(id, order_id, type, message, created_at)`

Each service runs its own migrations on startup (or via an init job). Use sqlc for queries.

---

## 7. Cross-cutting conventions

- **Graceful shutdown:** every service handles SIGTERM, drains in-flight work, closes Kafka/DB cleanly (matters for k8s rescheduling).
- **Health:** each service exposes `/healthz` (liveness) and `/readyz` (readiness — checks DB/Kafka). Wire these to k8s probes.
- **Config via env** only. Provide a `.env.example`. Key vars: `DATABASE_URL`, `KAFKA_BROKERS`, `GRPC_PORT`, `HTTP_PORT`, `LOG_LEVEL`, plus per-service upstream addresses (e.g. `PRODUCT_GRPC_ADDR`).
- **Idempotent consumers:** events may be redelivered; consumers must dedupe (e.g. on `order_id`).
- **Retries with backoff** on the producer and on consumer→DB writes, so a Kafka restart doesn't lose events.
- **Structured JSON logs** with a request/trace id propagated across gRPC calls.
- **One Dockerfile per service**, multi-stage, final image `FROM gcr.io/distroless/static` (or `alpine`), built for `linux/arm64`.

---

## 8. Infrastructure (Terraform) — build but DO NOT APPLY without my confirmation

- **network:** VPC with **public** subnets across 2 AZs (no NAT). Security groups: nodes ↔ nodes, nodes → Kafka:9092, nodes → RDS:5432, ALB → nodes. VPC gateway endpoint for S3.
- **compute:** launch template using a Graviton AMI (Amazon Linux 2023 arm64 or Ubuntu arm64). `user_data` installs k3s. One **server** node (on-demand, `t4g.small`) and an **ASG of agents** (`min=2`, `t4g.small`/`medium`, **Spot**). Agents join via the k3s token (store token in SSM Parameter Store, fetch in user-data). Tag everything for cost tracking.
- **kafka:** one `aws_instance` (`t4g.small`/`medium`, Graviton) + `aws_ebs_volume` (gp3, ~20GB) mounted at the Kafka log dir. `user_data` installs Docker and runs Apache Kafka KRaft (or Redpanda). **Critical:** set `advertised.listeners` to the instance's private DNS/IP — never `localhost`. Create a Route53 **private hosted zone** record like `kafka.internal` so services use a stable address; services get `KAFKA_BROKERS=kafka.internal:9092`. SG allows 9092 only from the node SG.
- **rds:** single-AZ Postgres `db.t4g.micro`, in the VPC, SG allows 5432 from nodes only. Credentials in SSM Parameter Store (SecureString) or Secrets Manager; injected into k8s as a Secret.
- **alb:** ALB or NLB fronting Traefik (k3s NodePort) for the `bff` ingress + the frontend if served from cluster (or serve frontend from S3+CloudFront — your call, document it).
- **ecr:** one repo per service.
- **iam:** GitHub OIDC provider + a deploy role scoped to ECR push + EC2 describe + SSM read, assumed by Actions.
- **State:** S3 bucket + DynamoDB lock table (bootstrap these first).
- **Guardrails:** an `aws_budgets_budget` alarm at ~$5 and ~$20; a `make destroy` that tears the whole env down.

---

## 9. Kubernetes / Helm

- One Helm chart per service under `deploy/charts/`, plus an umbrella chart for one-command install.
- For services that must survive a node failure (`bff`, `product`, `order`): `replicas: 2`, `topologySpreadConstraints` (or `podAntiAffinity`) so replicas land on **different nodes**, plus a `PodDisruptionBudget`. Leave node headroom so survivors can absorb evicted pods.
- Liveness/readiness probes wired to `/healthz` / `/readyz`.
- Resource `requests`/`limits` sized for small nodes (e.g. 50m CPU / 64–128Mi per Go service).
- DB URL and Kafka brokers injected from a k8s Secret/ConfigMap.
- Optional HPA on CPU for `bff`/`order`.
- Traefik `IngressRoute` for the `bff` REST API.

**Tip:** Kafka is NOT in the cluster — services reach it at `kafka.internal:9092` over the VPC. No StatefulSet, no EBS CSI needed for Kafka.

---

## 10. CI/CD (GitHub Actions, OIDC)

- **ci.yml** (on PR): `buf lint`, `buf breaking`, `go test ./...`, `golangci-lint run`, `terraform fmt -check` + `validate`, frontend `npm run lint && build`.
- **deploy.yml** (on merge to `main`): assume AWS role via OIDC → build **arm64** images with `docker buildx` → push to ECR → `helm upgrade --install` each chart against the cluster (kubeconfig fetched from the server node via SSM, or stored as a masked secret). Run `terraform plan` and post it on the PR; gate `apply` behind manual approval.

---

## 11. Local development (do this BEFORE any AWS)

`docker-compose.yml` must bring up: Postgres, a single Kafka (KRaft) or Redpanda, and all six services + run migrations. `make up` starts everything; the Next.js app talks to `bff` on localhost. This is the primary dev loop — the cloud should be a deployment target, not a dependency for iterating.

---

## 12. Build plan (execute in order; stop & verify after each phase)

**Phase 0 — Scaffolding.** Repo layout, Go module, `Makefile`, `buf.yaml`/`buf.gen.yaml`, `golangci-lint` config, empty `docker-compose.yml`, CI skeleton, README stub. ✅ *Verify:* `make` targets run; `buf lint` passes on an empty proto.

**Phase 1 — Protos + shared pkg.** Define all protos, generate `gen/`. Implement `pkg/`: config, slog logger, pgx pool + migrate runner, franz-go producer/consumer helpers, health server. ✅ *Verify:* code generates and compiles; `pkg` unit tests pass.

**Phase 2 — Product service.** gRPC server + Postgres (sqlc) + migrations + Dockerfile. Add to compose. ✅ *Verify:* `ListProducts`/`GetProduct` work locally (grpcurl), seeded data returned.

**Phase 3 — BFF + frontend skeleton.** `bff` exposes REST via grpc-gateway, calls `product` over gRPC. Next.js page lists products from the `bff`. ✅ *Verify:* browser shows a product list end-to-end via compose.

**Phase 4 — Order service + Kafka producer.** `PlaceOrder` writes an order to Postgres and produces `OrderPlaced`. ✅ *Verify:* placing an order persists it and a message lands on `orders.placed` (inspect with a console consumer).

**Phase 5 — Consumers.** `inventory`, `payment`, `notification` consume events, are idempotent, retry with backoff, write to their schemas, and emit follow-on events. ✅ *Verify:* an order flows OrderPlaced → stock reserved → payment completed → notification recorded.

**Phase 6 — Frontend order flow.** Cart, place order, live order-status view (poll the notification/order read API). ✅ *Verify:* full happy path in the browser locally.

**Phase 7 — Terraform (no apply yet).** Write all modules + `envs/dev`. Bootstrap remote state. ✅ *Verify:* `terraform validate` + `plan` clean. **Pause for my go-ahead before `apply`.**

**Phase 8 — Helm.** Charts for every service with replicas/anti-affinity/PDB/probes; umbrella chart. ✅ *Verify:* `helm template`/`lint` clean; (after infra) deploys to the cluster.

**Phase 9 — CI/CD.** `ci.yml` + `deploy.yml` with OIDC, arm64 buildx, ECR push, helm deploy, gated terraform. ✅ *Verify:* PR runs checks; a merge deploys.

**Phase 10 — Harden & document.** OTel traces, HPA, budget alarm, `make destroy`, README with architecture diagram, cost notes, the single-broker/single-AZ tradeoffs, and a "node-failure HA demo" runbook (kill an agent, watch pods reschedule). ✅ *Verify:* docs let a stranger run it; destroy leaves no billable resources.

---

## 13. Definition of done

- `make up` runs the whole stack locally; the storefront completes an order end-to-end.
- Terraform provisions a 3-node k3s cluster (1 server + 2 spot agents) + standalone Kafka EC2 + single-AZ RDS, all in one VPC, **no NAT/MSK/EKS/Multi-AZ**.
- Killing a worker node reschedules pods; `bff`/`product`/`order` stay available (≥2 replicas, spread).
- CI is green; merge to `main` builds arm64 images and deploys via Helm.
- `make destroy` removes everything; a budget alarm is in place.
- README documents architecture, costs, tradeoffs, and how to demo node-failure HA.

---

## 14. When in doubt

Prefer the cheap, simple, well-documented option over the "correct enterprise" one, and **write down the tradeoff** instead of silently upgrading to something billable. If a step would cost money or isn't clearly specified here, ask me before doing it.
