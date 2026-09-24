# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Goal

Rebuild the existing soda machine tracking prototype (Node.js/AngularJS/MongoDB in `consumables/`) as a modern, production-ready system called **SodaTracker** (in `sodaTracker/`). The new system must support dual deployment: self-hosted (Docker Compose for demos) and AWS serverless (Lambda + DynamoDB + SNS).

## Legacy System (reference only)

The original prototype lives in `consumables/` and `sodaTracker/`:

- `consumables/api/` — LoopBack 2.x REST API (Node.js, MongoDB)
- `consumables/web/` — AngularJS 1.2 dashboard (Gulp, Bower, AmCharts)
- `consumables/soda-tracker/` — GPIO monitor for soda machine buttons
- `consumables/bin-button/ButtonPi/` — GPIO monitor for button box
- `consumables/barcode-scanner/` — Placeholder for scanner input
- `sodaTracker/` — Legacy soda tracker + quick2wire-gpio-admin (C GPIO utility)

Note: Some legacy debris (`sodaTracker.js`, `npm-debug.log`, `quick2wire-gpio-admin/`, root-level `node_modules/`) still exists inside `sodaTracker/` and should be cleaned up.

## SodaTracker Tech Stack

- **Backend:** Go with `go-chi/chi` router + `go-chi/cors` middleware
- **Dual deployment:** Same chi router runs as standalone HTTP server (self-hosted) OR as Lambda via `aws-lambda-go-api-proxy/chi` (Phase 4)
- **Database:** DynamoDB (AWS, Phase 4) / SQLite via `modernc.org/sqlite` (self-hosted, pure Go, no CGO) — behind a `Store` interface
- **Events:** SNS (AWS) / in-process channel-based pub/sub (self-hosted) — behind an `EventBus` interface (Phase 3)
- **Frontend:** React 19 + TypeScript + Vite + TanStack Query + Recharts + Tailwind CSS v4 (Vite plugin, no `tailwind.config.js`) + react-router-dom v7
- **Pi agent:** Go binary cross-compiled for ARM; real `periph.io` GPIO behind the `rpi` build tag (mock otherwise), SQLite local cache. Ships inside the same container image as the server.
- **IaC:** OpenTofu (Terraform-compatible) for AWS infrastructure (DynamoDB, Lambda, API Gateway, S3, CloudFront, SNS)
- **Auth:** JWT (HS256, 24h expiry) for admin dashboard, API key for Pi agent, `APIKeyOrJWT` middleware on shared admin routes
- **IDs:** Server-generated UUIDs via `google/uuid`

## SodaTracker Repository Structure

```
sodaTracker/
├── cmd/
│   ├── server/main.go        # Self-hosted HTTP server
│   ├── lambda/main.go        # AWS Lambda entry point (chi adapter v2)
│   ├── piagent/main.go        # Raspberry Pi GPIO agent
│   ├── slackbot/              # Slack bot (Phase 3, empty stub)
│   └── discordbot/            # Discord bot (Phase 3, empty stub)
├── internal/
│   ├── api/                   # chi router, middleware, handlers
│   │   └── docs/              # Generated Swagger/OpenAPI docs (swaggo/swag)
│   ├── domain/                # Entity structs (Product, Machine, Slot, Dispense, Restock, User)
│   ├── store/                 # Store interface + sqlite/ and dynamo/ implementations
│   ├── events/                # EventBus interface + local/ and sns/ implementations
│   ├── service/               # Business logic (dispense, restock, stock calculation, login, stats)
│   ├── seed/                  # Admin user seeding (shared between server and Lambda)
│   ├── notify/                # Slack and Discord notification clients (Phase 3)
│   ├── auth/                  # API key middleware + JWT middleware (auth.go, jwt.go)
│   ├── config/                # Env-based config loading
│   ├── logging/               # slog setup (JSON/text handler, LOG_FORMAT env)
│   └── export/                # Dispense report export (CSV, PDF)
├── pkg/
│   ├── gpio/                  # GPIO interface + mock implementation (rpi build tag for real GPIO)
│   └── piagent/               # Agent logic, local SQLite cache, YAML config
├── web/                       # React frontend (Vite + Tailwind)
│   └── src/
│       ├── pages/             # CustomerDashboard, LoginPage, AdminDashboard, MachineDetailPage, RestockPage, AlertsPage, SetupPage
│       ├── components/        # StockGrid, AdminLayout, ProtectedRoute
│       ├── context/           # AuthContext (JWT token + user state, localStorage persistence)
│       └── api/               # API client + TypeScript types
├── infra/                     # OpenTofu infrastructure (DynamoDB, Lambda, API GW, S3, CloudFront, SNS)
├── .github/workflows/         # CI: test + multi-arch image build/push to GHCR
├── docker-compose.yml         # Self-hosted: single container, SQLite volume, GPIO devices
├── docker-entrypoint.sh       # Mode dispatch: all | server | piagent
├── Dockerfile                 # Multi-stage cross-compile: server + piagent (rpi tag) + React
├── Makefile
├── piagent.yaml.example       # Example Pi agent config (pin-to-slot mappings)
└── go.mod                     # Module: github.com/jeffwnelson/sodatracker
```

## Development Commands

All commands run from `sodaTracker/`.

### API Server
```bash
make build            # Build server + piagent binaries to bin/
make build-server     # Build only the server binary
make build-piagent    # Build only the piagent binary (current platform)
make run              # Build and run API server on :3000
make test             # Run all Go tests (go test ./...)
make lint             # Run golangci-lint
make clean            # Remove bin/ directory
```

### Running Specific Tests
```bash
# Run tests for a single package
go test ./internal/api/...
go test ./internal/store/sqlite/...

# Run a single test by name
go test ./internal/api/... -run TestFullWorkflow
go test ./internal/store/sqlite/... -run TestDispenseAndRestock

# Verbose output
go test -v ./internal/api/... -run TestHealth
```

### Web Dashboard (`sodaTracker/web/`)
```bash
make web-install    # npm install
make web-dev        # Vite dev server on :4000, proxies /api to :3000
make web-build      # tsc -b && vite build → production build to web/dist/
```

Note: `make web-build` runs TypeScript type checking first — TS errors will fail the build. The frontend also has `eslint .` available via `npm run lint` in `web/` but no corresponding Makefile target.

### Docker
```bash
make docker-up      # docker compose up --build -d
make docker-down    # docker compose down
```

### Lambda (AWS)
```bash
make build-lambda        # Cross-compile for Lambda (linux/arm64), creates bin/lambda.zip
make tf-plan             # OpenTofu plan (cd infra && tofu plan)
make tf-apply            # OpenTofu apply (cd infra && tofu apply)
make deploy-frontend     # Build frontend, sync to S3, invalidate CloudFront
```

### Pi Agent
```bash
make build-piagent-arm   # Cross-compile for Raspberry Pi (linux/arm7)
./bin/piagent -config piagent.yaml   # Run with config file
```

### Swagger / OpenAPI Docs
```bash
make swagger          # Regenerate Swagger docs (requires swag CLI: go install github.com/swaggo/swag/v2/cmd/swag@latest)
```

Swagger UI is served at `/swagger/index.html` when the server is running. Docs are generated from Go comment annotations on handlers (`internal/api/handlers.go`) and the general API info block in `internal/api/router.go`. Generated files (`internal/api/docs/`) are committed to git so the Dockerfile works without installing `swag`.

### Pi Deployment (GHCR → Raspberry Pi)

The self-hosted demo runs on a Raspberry Pi 3 Model B (arm64) as a **single Docker
container running both the API server and the GPIO pi agent**.

- **SSH:** `ssh pi@<office-ip>` (password: `<redacted>`)
- **GHCR image:** `ghcr.io/jeffwnelson/sodatracker:latest` (linux/amd64 + linux/arm64)
- **Container name:** `sodatracker`
- **Compose dir on Pi:** `/home/pi/sodatracker` (git clone of this repo)
- **Data volume:** `sodatracker-data` at `/data` (SQLite DB + piagent cache)
- **Dashboard URL:** `http://<office-ip>:3000`

Note: `sodatracker.example.com` no longer resolves — use the IP.

**CI publishes the image.** `.github/workflows/docker-publish.yml` runs
`go build`/`go vet`/`go test` plus a `-tags rpi` compile check, then builds and
pushes a multi-arch image on every push to `main`. Tags: `latest`,
`sha-<commit>`, and `v*` git tags.

```bash
make pi-deploy      # ssh to Pi, docker compose pull && docker compose up -d
make pi-logs        # tail container logs
make docker-push    # manual arm64 build+push, bypassing CI
```

GHCR auth on the Pi is already configured (`~pi/.docker/config.json`). If it expires:
```bash
gh auth token | sshpass -p '<redacted>' ssh pi@<office-ip> 'cat | docker login ghcr.io -u jeffwnelson --password-stdin'
```

### Container Modes and GPIO

`docker-entrypoint.sh` is the image ENTRYPOINT and dispatches on its first argument:

| Mode | Behavior |
|------|----------|
| `all` (default CMD) | Starts `server`, waits for `/api/v1/health`, then starts `piagent`. Exits if either child dies so the restart policy takes over. |
| `server` | `exec`s the API server only — for hosts without GPIO. |
| `piagent` | `exec`s the agent only — for pointing at a remote API via `PIAGENT_API_URL`. |
| anything else | `exec "$@"` (escape hatch for a shell). |

`tini` is PID 1 so signals reach both children; `docker stop` returns in ~1s with exit 0.

**GPIO requirements.** periph.io memory-maps `/dev/gpiomem` on BCM283x, so the
container needs `--device /dev/gpiomem` (plus `/dev/gpiochip0` as the character-device
fallback). `docker-compose.yml` maps both. The container runs as root, so no
`group_add` for the host `gpio` group (gid 986) is needed. The entrypoint fails
fast with a clear message if the config or GPIO devices are missing.

The `piagent` binary in the image is **always built with `-tags rpi`** (real
periph.io GPIO, not the mock). That tag compiles cleanly on amd64 too, so the
multi-arch build works — it just finds no pins there.

**Pi agent config** is mounted at `/config/piagent.yaml` (`PIAGENT_CONFIG` env var).
Pin mappings are YAML-only. Deployment-specific fields have env overrides
(`applyEnvOverrides` in `pkg/piagent/config.go`): `PIAGENT_API_URL`,
`PIAGENT_API_KEY`, `PIAGENT_MACHINE_ID`, `PIAGENT_DB_PATH`, `PIAGENT_POLL_MS`,
`PIAGENT_RETRY_MS`.

## Key Configuration

- **API port:** 3000 (env `PORT`)
- **SQLite DB path:** `sodatracker.db` (env `DB_PATH`) — opens with `?_journal_mode=WAL&_busy_timeout=5000`
- **API key:** `dev-api-key` (env `API_KEY`) — used by Pi agent; also accepted on admin CRUD routes via `APIKeyOrJWT` middleware
- **JWT secret:** `dev-jwt-secret` (env `JWT_SECRET`) — signs HS256 tokens for admin dashboard auth
- **Admin password:** `dev-admin-password` (env `ADMIN_PASSWORD`) — bcrypt-hashed on first startup to seed the `admin` user
- **Static file dir:** empty in dev, `/app/static` in Docker (env `STATIC_DIR`) — serves SPA with index.html fallback
- **Frontend dev server:** port 4000 with proxy to API at :3000
- **DynamoDB table name:** (env `TABLE_NAME`) — only set for Lambda deployment
- **SNS topic ARN:** (env `SNS_TOPIC_ARN`) — only set for Lambda deployment, enables SNS event fan-out
- **Log format:** `text` for local dev, `json` for Docker/Lambda (env `LOG_FORMAT`, default `"text"`)
- **Pi agent config:** YAML at `/config/piagent.yaml` (env `PIAGENT_CONFIG`) — see `piagent.yaml.example`. Pin mappings are YAML-only; other fields accept `PIAGENT_*` env overrides
- **Pi agent cache DB:** `piagent-cache.db` (default, configurable in YAML)

## Data Model

**Domain entities:** Product (upc, name, costPerCan, imageUrl), Machine (name, location, slotCount), Slot (machineId, position, productId, gpioPin, capacity, lowThreshold), Dispense (machineId, productId, slotPosition, sourceType, timestamp), Restock (machineId, userId, slots[], timestamp), User (username, passwordHash `json:"-"`, role)

**Stock calculation:** Computed on read — last restock quantity minus dispenses since that restock. No stored counter (avoids write contention). Stock is capped at 0 minimum (never goes negative).

**Slot status thresholds:** `currentStock > lowThreshold` = "ok" (green), `currentStock > 0 && <= lowThreshold` = "low" (yellow), `currentStock <= 0` = "empty" (red)

## API Routes

| Method | Path | Auth | Purpose | Status |
|--------|------|------|---------|--------|
| GET | `/api/v1/health` | None | Health check | Done |
| GET | `/api/v1/machines` | None | List all machines | Done |
| GET | `/api/v1/machines/:id/status` | None | Machine stock status | Done |
| POST | `/api/v1/auth/login` | None | Login (returns JWT) | Done |
| POST | `/api/v1/dispenses` | API Key | Register dispense (from Pi) | Done |
| POST | `/api/v1/machines` | API Key or JWT | Create machine | Done |
| POST | `/api/v1/machines/:id/slots` | API Key or JWT | Configure slots | Done |
| POST | `/api/v1/machines/:id/restock` | API Key or JWT | Log restock | Done |
| GET | `/api/v1/machines/:id/dispenses` | API Key or JWT | Dispense history | Done |
| GET | `/api/v1/machines/:id/restocks` | API Key or JWT | Restock history | Done |
| GET | `/api/v1/products` | API Key or JWT | List products | Done |
| POST | `/api/v1/products` | API Key or JWT | Create product | Done |
| DELETE | `/api/v1/products/:id` | API Key or JWT | Delete product | Done |
| DELETE | `/api/v1/machines/:id` | API Key or JWT | Delete machine (cascade) | Done |
| GET | `/api/v1/admin/stats` | API Key or JWT | Dispense stats (aggregated) | Done |
| GET | `/api/v1/admin/cost` | API Key or JWT | Machine cost (30d) | Done |
| GET | `/api/v1/machines/:id/alerts` | API Key or JWT | List alert configs | Done |
| POST | `/api/v1/machines/:id/alerts` | API Key or JWT | Create alert config | Done |
| PUT | `/api/v1/alerts/:id` | API Key or JWT | Update alert config | Done |
| DELETE | `/api/v1/alerts/:id` | API Key or JWT | Delete alert config | Done |
| GET | `/api/v1/machines/:id/alert-events` | API Key or JWT | List fired alerts | Done |
| GET | `/api/v1/machines/:id/predictions` | API Key or JWT | Stock-out predictions | Done |
| GET | `/api/v1/reports/dispenses` | API Key or JWT | Export CSV/PDF (`?format=csv\|pdf`) | Done |
| CRUD | `/api/v1/webhooks` | JWT | Webhook subscriptions | Phase 3 |

**Auth:** API key middleware accepts both `X-API-Key: <key>` and `Authorization: Bearer <key>` headers. The `APIKeyOrJWT` middleware on admin CRUD routes accepts either an API key or a valid JWT token, so both the Pi agent and the admin dashboard can call them.

## Architecture Details

- **JWT auth** uses HS256 signing with 24h token expiry. Claims include `userId`, `username`, `role`. Tokens are generated by `auth.GenerateToken()`, validated by `JWTMiddleware`. The `APIKeyOrJWTMiddleware` tries API key first (X-API-Key header), then JWT from Authorization Bearer header (`internal/auth/jwt.go`).
- **Admin user seeding** — On startup, if no "admin" user exists, one is created with bcrypt-hashed password from `ADMIN_PASSWORD` env var (`cmd/server/main.go`).
- **API key auth** checks `X-API-Key` header first, falls back to `Authorization: Bearer` prefix stripping (`internal/auth/auth.go`)
- **SetSlots** (`POST /machines/:id/slots`) is a **destructive replace** — deletes all existing slots for the machine, then inserts the new set in a transaction. Not a merge/patch.
- **CreateProduct and CreateMachine** are upserts (`ON CONFLICT DO UPDATE`). CreateDispense is a plain INSERT.
- **Restock slots** are stored as a JSON blob (`slots_json TEXT`) in the `restocks` table, not as normalized rows.
- **List endpoints** return `[]` (empty array) instead of `null` when no results — nil slices are converted before JSON encoding.
- **Error responses** always use the format `{"error": "<message>"}` via `writeErr()`.
- **Default dispense sourceType** is `"gpio"` if not provided in the request body.
- **Client-supplied IDs** are accepted but optional — server generates UUIDs if omitted.
- **Static file serving** includes SPA fallback (unknown paths serve `index.html`).
- **Dockerfile** builds with `CGO_ENABLED=0` — this is why `modernc.org/sqlite` (pure Go) is used instead of `mattn/go-sqlite3`.
- **In-memory aggregation** — `GetDispenseStats` fetches dispenses via `ListDispenses` and aggregates in Go (hourly bucketing, product ranking by count). No SQL GROUP BY queries — acceptable for current scale.
- **Service owns JWT signing** — `service.Login()` calls `auth.GenerateToken()` internally. JWT secret is passed to the Service constructor.
- **EventBus** — `events.EventBus` interface (`internal/events/events.go`) with `Publish`, `Subscribe`, `Close`. Local implementation (`internal/events/local/`) uses synchronous in-process dispatch with `sync.RWMutex`. Event data uses `map[string]any` to stay generic for future SNS/webhook delivery. Bus is nil-safe — service skips publish if bus is nil.
- **Alert evaluation flow** — `RegisterDispense` → publishes `EventDispense` → subscriber calls `EvaluateStockAlert` → computes stock → checks alert configs (slot-specific > machine-wide `slotPosition=0` > slot's own `lowThreshold`) → creates `AlertEvent` if threshold breached and no active alert exists (deduplication via `GetActiveAlert`). `CreateRestock` clears active alerts for restocked slots.
- **AlertConfig `slotPosition=0`** means machine-wide default. `LowThreshold=-1` means use the slot's own threshold. Stored with `UNIQUE(machine_id, slot_position)` constraint (upsert on create).
- **Predictive stock-out** — `GetStockPredictions` computes `dispensesPerDay = count / lookbackDays` (default 14 days) per slot, then `estimatedDaysLeft = currentStock / dispensesPerDay`. Returns -1 if no dispense data.
- **Frontend token storage** — localStorage with key `soda_token`. AuthContext initializes from localStorage so page refresh preserves login state.
- **Frontend refresh intervals:** machine list refreshes every 30s, individual machine status cards every 10s, admin stats staleTime is 30s, TanStack Query default staleTime is 5s.
- **TypeScript strict mode** enabled with `verbatimModuleSyntax` — all type imports must use `import type`.
- **Structured logging** — All Go code uses `log/slog` (no `log` package). `internal/logging/Setup()` configures JSON or text handler based on `LOG_FORMAT` env var (default `"text"`, Docker/Lambda set `"json"`). Custom request logging middleware replaces chi's `middleware.Logger`, logs method/path/status/duration/requestId via slog. `middleware.RequestID` generates unique request IDs for tracing.
- **Dispense report export** — `GET /api/v1/reports/dispenses?machineId=X&format=csv|pdf` exports dispense history. CSV uses stdlib `encoding/csv`. PDF uses `go-pdf/fpdf` (pure Go, no CGO). Both enrich dispenses with product name/UPC/cost from a product lookup map. PDF includes title, date range, table, and summary footer.
- **DynamoDB single-table design** — Table `SodaTracker-{env}`, PK/SK (both S), GSI1 (GSI1PK/GSI1SK, projection ALL). Entity types: Product, Machine, Slot, Dispense, Restock, User, AlertConfig, AlertEvent. Key patterns use `#`-delimited composite keys with RFC3339Nano timestamps for lexicographic sort. AlertEvent GSI1PK switches from `ACTIVEALERT#...` to `CLEARED#...` when cleared (enables efficient active alert queries).
- **Lambda deployment** — Single Lambda (ARM64/Graviton, 256MB, provided.al2023) serving all routes via `aws-lambda-go-api-proxy/chi` V2 adapter. HTTP API v2 (API Gateway) routes `ANY /api/{proxy+}` to Lambda. CloudFront serves S3 frontend with `/api/*` behavior forwarding to API Gateway. Admin seeding (`internal/seed/`) runs on cold start.
- **SNS publisher** — Optional SNS event fan-out registered as a subscriber on the local EventBus. Publishes all event types with `eventType` message attribute for filtering. Only active when `SNS_TOPIC_ARN` env var is set.
- **Swagger/OpenAPI** — Generated by `swaggo/swag` v2 from Go comment annotations. `internal/api/router.go` has the general API info block (title, version, security definitions). Each handler in `handlers.go` has `@Summary`, `@Description`, `@Tags`, `@Param`, `@Success`/`@Failure`, `@Security`, and `@Router` annotations. `internal/api/dto.go` defines named response types (`ErrorResponse`, `HealthResponse`, `LoginRequest`, `LoginResponse`, `DeleteResponse`, `CostResponse`) for swag reference. Domain and service types have `example` struct tags. Generated docs (`internal/api/docs/`) are committed. Swagger UI served at `/swagger/*` via `swaggo/http-swagger/v2`. Regenerate with `make swagger`.
- **OpenTofu infrastructure** — `infra/` directory with S3 backend state, DynamoDB table, Lambda function, HTTP API v2, SNS topic, S3 + CloudFront frontend. AWS account 136532201934, region us-west-2, profile "personal". Sensitive vars: api_key, jwt_secret, admin_password.

## Test Structure

Tests use **in-memory SQLite** (`:memory:`) — no external dependencies needed. **40 tests total** across 6 packages. DynamoDB integration tests (6 tests, `//go:build integration` tag) require DynamoDB Local.

- `internal/events/local/local_test.go` — EventBus tests (5): `TestPublishSubscribe`, `TestMultipleSubscribers`, `TestUnsubscribe`, `TestPublishNoSubscribers`, `TestDifferentEventTypes`
- `internal/store/sqlite/sqlite_test.go` — Store CRUD tests (6): `TestProductCRUD`, `TestMachineSlotsAndStatus`, `TestUserCRUD`, `TestAlertConfigCRUD`, `TestAlertEventCRUD`, `TestDispenseAndRestock`
- `internal/auth/jwt_test.go` — JWT tests (4): `TestGenerateAndValidateToken`, `TestJWTMiddleware_MissingToken`, `TestJWTMiddleware_InvalidToken`, `TestJWTMiddleware_ValidToken`
- `internal/service/service_test.go` — Service logic tests (6): `TestLogin` (success + bad password + non-existent user), `TestGetDispenseStats`, `TestGetMachineCost`, `TestEvaluateStockAlert`, `TestCreateRestockClearsAlerts`, `TestGetStockPredictions`
- `internal/api/handlers_test.go` — API integration tests (16) using `httptest.NewRecorder()` against the real chi router with EventBus wired: `TestHealth`, `TestCreateAndListMachines`, `TestDispenseRequiresAPIKey`, `TestLoginSuccess`, `TestLoginBadPassword`, `TestAdminStatsRequiresAuth`, `TestFullWorkflow`, `TestFullWorkflowWithJWT`, `TestAlertConfigCRUD`, `TestPredictionsEndpoint`, `TestFullWorkflowWithAlerts`, `TestExportDispensesCSV`, `TestExportDispensesPDF`, `TestDeleteProduct`, `TestDeleteMachine`, `TestExportRequiresAuth`
- `internal/export/export_test.go` — Export unit tests (3): `TestWriteDispensesCSV`, `TestWriteDispensesPDF`, `TestEmptyExport`
- `internal/store/dynamo/dynamo_test.go` — DynamoDB integration tests (6, `//go:build integration`): `TestProductCRUD`, `TestMachineSlotsAndStatus`, `TestDispenseAndRestock`, `TestUserCRUD`, `TestAlertConfigCRUD`, `TestAlertEventCRUD`. Requires DynamoDB Local (`docker run -p 8000:8000 amazon/dynamodb-local`), run with `go test -tags integration ./internal/store/dynamo/...`

## Key Business Logic

- `internal/service/service.go` — `GetMachineStatus`: computes stock per slot (restock qty - dispenses since restock), assigns status (ok/low/empty), caps stock at 0. `Login`: bcrypt verify + JWT generation. `GetDispenseStats`: in-memory aggregation (hourly bucketing, product ranking, cost). `GetMachineCost`: sum of costPerCan for all dispenses. `RegisterDispense`: store + publish EventDispense. `CreateRestock`: store + clear alerts + publish EventStockCleared. `EvaluateStockAlert`: compute stock, resolve threshold from configs, deduplicate, create AlertEvent. `GetStockPredictions`: dispenses/day over lookback window, estimate days until stock-out.
- `internal/api/handlers.go` — `createDispense`: auto-resolves productId from slot config if not provided. `login`: authenticates and returns JWT. `getAdminStats`, `getMachineCost`, `listRestocks`: admin analytics. Alert CRUD: `listAlertConfigs`, `createAlertConfig`, `updateAlertConfig`, `deleteAlertConfig`. `listAlertEvents`: fired alerts with `?active=true` filter. `getStockPredictions`: per-slot predictions with `?lookbackDays` param. `exportDispenses`: CSV/PDF report with `?machineId&format=csv|pdf` params.
- `pkg/piagent/agent.go` — polls GPIO pins every 100ms, detects button release (1→0 transition for active-low buttons), sends to API, caches failures in local SQLite for retry every 30s (max 50 pending events per retry cycle, stops on first failure)
- `internal/store/sqlite/sqlite.go` — `GetLastRestock`: walks restocks in reverse chronological order to find the most recent one containing the target slot position

## Phased Implementation Plan

### Phase 1 — Foundation [COMPLETE]
- [x] Repo scaffolding, Go modules, Makefile
- [x] Domain types (`internal/domain/`)
- [x] Env-based config (`internal/config/`)
- [x] Store interface + SQLite implementation with migrations (`internal/store/`)
- [x] Business logic service: stock calculation, dispense registration (`internal/service/`)
- [x] API key auth middleware (`internal/auth/`)
- [x] Core API: health, machines, products, slots, dispenses, restocks (`internal/api/`)
- [x] Self-hosted server with graceful shutdown (`cmd/server/`)
- [x] Pi agent: GPIO interface + mock, polling, button release detection, local SQLite cache, retry, YAML config (`pkg/gpio/`, `pkg/piagent/`, `cmd/piagent/`)
- [x] Customer dashboard: React + TypeScript + Vite + Tailwind + TanStack Query, stock grid with auto-refresh (`web/`)
- [x] Docker Compose: multi-stage Dockerfile, single container serving API + static frontend
- [x] Tests: SQLite store CRUD tests, API integration tests including full workflow

### Phase 2 — Admin & Restocking [COMPLETE]
- [x] JWT auth: login endpoint (`/auth/login`), HS256 token generation (24h expiry), `JWTMiddleware`, `APIKeyOrJWTMiddleware` (`internal/auth/jwt.go`)
- [x] User domain type + store methods (`CreateUser`, `GetUserByUsername`) + users table migration
- [x] Auto-seed admin user on startup (bcrypt hash, `ADMIN_PASSWORD` env var)
- [x] Service methods: `Login` (bcrypt verify + JWT), `GetDispenseStats` (in-memory aggregation), `GetMachineCost`, `ListRestocks`
- [x] Admin API endpoints: `GET /admin/stats`, `GET /admin/cost`, `GET /machines/:id/restocks`
- [x] Admin CRUD routes upgraded to `APIKeyOrJWTMiddleware` (accepts both API key and JWT)
- [x] Frontend auth: `AuthContext` (localStorage token persistence), `ProtectedRoute`, `AdminLayout` with nav
- [x] Login page (`/admin/login`): username/password form with error handling
- [x] Admin dashboard page (`/admin`): machine selector, stats summary cards, Recharts time-series LineChart, top products table, machine list
- [x] Machine detail page (`/admin/machines/:id`): RadialBarChart stock gauges, cost card, dispense timeline, restock history table
- [x] Restock page (`/admin/machines/:id/restock`): per-slot quantity inputs, submit mutation, query invalidation
- [x] Cost tracking (costPerCan * dispenses, aggregated in GetDispenseStats and GetMachineCost)
- [x] Tests: 15 total — JWT auth (4), service logic (3), store CRUD (4), API integration (4 including TestFullWorkflowWithJWT)

### Phase 3 — Events & Notifications [COMPLETE]
- [x] EventBus interface + local (in-process channel) implementation (`internal/events/`, `internal/events/local/`)
- [x] Alert service: evaluates stock after each dispense, fires low_stock/out_of_stock events, deduplicates, clears on restock
- [x] Alert config CRUD: per-machine/slot threshold configuration (store, service, API, UI)
- [x] Alert configuration UI (`/admin/alerts`): active alerts, config management
- [x] Predictive stock-out: linear regression on dispense rate, shown in alerts page + machine detail page
- [x] Tests: 32 total — EventBus (5), store (6), auth (4), service (6), API (11)
- [ ] Slack bot: push notifications on events, pull commands (`/soda status`, `/soda top`)
- [ ] Discord bot: `discordgo` persistent WebSocket, same push/pull features
- [ ] Webhook system: CRUD + HMAC-signed delivery

### Phase 4 — AWS Deployment [COMPLETE]
- [x] Config updates: `TABLE_NAME`, `SNS_TOPIC_ARN` env vars (`internal/config/`)
- [x] Admin seeding extracted to shared package (`internal/seed/`) for server + Lambda reuse
- [x] DynamoDB store implementation: single-table design, PK/SK/GSI1 patterns, all 35 Store methods (`internal/store/dynamo/`)
- [x] DynamoDB integration tests (6 tests, `//go:build integration` tag)
- [x] SNS event publisher: forwards events from local bus to SNS topic (`internal/events/sns/`)
- [x] Lambda entry point: chi V2 adapter, DynamoDB store, local bus + optional SNS fan-out (`cmd/lambda/`)
- [x] OpenTofu infrastructure: DynamoDB, Lambda (ARM64), HTTP API v2, SNS, S3, CloudFront, IAM (`infra/`)
- [x] Build targets: `make build-lambda`, `make deploy-frontend`, `make tf-plan`, `make tf-apply`
- [x] CI/CD via GitHub Actions (test + multi-arch GHCR publish)

### Phase 5 — Polish [IN PROGRESS]
- [x] Structured logging: `log/slog` replaces `log` across all Go code, `LOG_FORMAT` env var (`text`/`json`)
- [x] Request ID middleware (`middleware.RequestID`) + custom structured request logging
- [x] CSV/PDF export for dispense reports (`GET /api/v1/reports/dispenses?format=csv|pdf`)
- [x] Swagger/OpenAPI docs via `swaggo/swag` — interactive UI at `/swagger/index.html`, all 23 endpoints annotated
- [ ] Mobile responsive polish
- [x] Pi agent runs in-container (no systemd unit needed; Docker restart policy is the watchdog)
- [ ] CloudWatch custom metrics
- [ ] E2E tests (Playwright)
