# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Goal

Rebuild the existing soda machine tracking prototype (Node.js/AngularJS/MongoDB in `consumables/`) as a modern, production-ready system called **SodaTracker** (in `sodatracker/`). The new system must support dual deployment: self-hosted (Docker Compose for demos) and AWS serverless (Lambda + DynamoDB + SNS).

## Legacy System (reference only)

The original prototype lives in `consumables/` and `sodaTracker/`:

- `consumables/api/` — LoopBack 2.x REST API (Node.js, MongoDB)
- `consumables/web/` — AngularJS 1.2 dashboard (Gulp, Bower, AmCharts)
- `consumables/soda-tracker/` — GPIO monitor for soda machine buttons
- `consumables/bin-button/ButtonPi/` — GPIO monitor for button box
- `consumables/barcode-scanner/` — Placeholder for scanner input
- `sodaTracker/` — Legacy soda tracker + quick2wire-gpio-admin (C GPIO utility)

Note: Some legacy debris (`sodaTracker.js`, `npm-debug.log`, `quick2wire-gpio-admin/`, root-level `node_modules/`) still exists inside `sodatracker/` and should be cleaned up.

## SodaTracker Tech Stack

- **Backend:** Go with `go-chi/chi` router + `go-chi/cors` middleware
- **Dual deployment:** Same chi router runs as standalone HTTP server (self-hosted) OR as Lambda via `aws-lambda-go-api-proxy/chi` (Phase 4)
- **Database:** DynamoDB (AWS, Phase 4) / SQLite via `modernc.org/sqlite` (self-hosted, pure Go, no CGO) — behind a `Store` interface
- **Events:** SNS (AWS) / in-process channel-based pub/sub (self-hosted) — behind an `EventBus` interface (Phase 3)
- **Frontend:** React 19 + TypeScript + Vite + TanStack Query + Recharts + Tailwind CSS v4 (Vite plugin, no `tailwind.config.js`) + react-router-dom v7
- **Pi agent:** Go binary cross-compiled for ARM, mock GPIO implementation (real `periph.io` GPIO not yet implemented — build tag `rpi` reserved), SQLite local cache
- **IaC:** AWS CDK in Go (Phase 4)
- **Auth:** JWT (HS256, 24h expiry) for admin dashboard, API key for Pi agent, `APIKeyOrJWT` middleware on shared admin routes
- **IDs:** Server-generated UUIDs via `google/uuid`

## SodaTracker Repository Structure

```
sodatracker/
├── cmd/
│   ├── server/main.go        # Self-hosted HTTP server
│   ├── lambda/                # AWS Lambda entry point (Phase 4, empty stub)
│   ├── piagent/main.go        # Raspberry Pi GPIO agent
│   ├── slackbot/              # Slack bot (Phase 3, empty stub)
│   └── discordbot/            # Discord bot (Phase 3, empty stub)
├── internal/
│   ├── api/                   # chi router, middleware, handlers
│   ├── domain/                # Entity structs (Product, Machine, Slot, Dispense, Restock, User)
│   ├── store/                 # Store interface + sqlite/ implementation (dynamo/ in Phase 4)
│   ├── events/                # EventBus interface (Phase 3) + sns/ and local/ implementations
│   ├── service/               # Business logic (dispense, restock, stock calculation, login, stats)
│   ├── notify/                # Slack and Discord notification clients (Phase 3)
│   ├── auth/                  # API key middleware + JWT middleware (auth.go, jwt.go)
│   └── config/                # Env-based config loading
├── pkg/
│   ├── gpio/                  # GPIO interface + mock implementation (rpi build tag for real GPIO)
│   └── piagent/               # Agent logic, local SQLite cache, YAML config
├── web/                       # React frontend (Vite + Tailwind)
│   └── src/
│       ├── pages/             # CustomerDashboard, LoginPage, AdminDashboard, MachineDetailPage, RestockPage
│       ├── components/        # StockGrid, AdminLayout, ProtectedRoute
│       ├── context/           # AuthContext (JWT token + user state, localStorage persistence)
│       └── api/               # API client + TypeScript types
├── infra/cdk/                 # AWS CDK stack (Phase 4, empty stub)
├── docker-compose.yml         # Self-hosted: single container, SQLite volume
├── Dockerfile                 # Multi-stage: Go API (CGO_ENABLED=0) + React static files
├── Makefile
├── piagent.yaml.example       # Example Pi agent config (pin-to-slot mappings)
└── go.mod                     # Module: github.com/jeffwnelson/sodatracker
```

## Development Commands

All commands run from `sodatracker/`.

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

### Web Dashboard (`sodatracker/web/`)
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

### Pi Agent
```bash
make build-piagent-arm   # Cross-compile for Raspberry Pi (linux/arm7)
./bin/piagent -config piagent.yaml   # Run with config file
```

## Key Configuration

- **API port:** 3000 (env `PORT`)
- **SQLite DB path:** `sodatracker.db` (env `DB_PATH`) — opens with `?_journal_mode=WAL&_busy_timeout=5000`
- **API key:** `dev-api-key` (env `API_KEY`) — used by Pi agent; also accepted on admin CRUD routes via `APIKeyOrJWT` middleware
- **JWT secret:** `dev-jwt-secret` (env `JWT_SECRET`) — signs HS256 tokens for admin dashboard auth
- **Admin password:** `dev-admin-password` (env `ADMIN_PASSWORD`) — bcrypt-hashed on first startup to seed the `admin` user
- **Static file dir:** empty in dev, `/app/static` in Docker (env `STATIC_DIR`) — serves SPA with index.html fallback
- **Frontend dev server:** port 4000 with proxy to API at :3000
- **Pi agent config:** YAML file only (no env var fallback) — see `piagent.yaml.example`
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
| GET | `/api/v1/admin/stats` | API Key or JWT | Dispense stats (aggregated) | Done |
| GET | `/api/v1/admin/cost` | API Key or JWT | Machine cost (30d) | Done |
| GET | `/api/v1/reports/dispenses` | JWT | Export CSV/PDF | Phase 5 |
| CRUD | `/api/v1/alerts` | JWT | Alert config | Phase 3 |
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
- **Frontend token storage** — localStorage with key `soda_token`. AuthContext initializes from localStorage so page refresh preserves login state.
- **Frontend refresh intervals:** machine list refreshes every 30s, individual machine status cards every 10s, admin stats staleTime is 30s, TanStack Query default staleTime is 5s.
- **TypeScript strict mode** enabled with `verbatimModuleSyntax` — all type imports must use `import type`.

## Test Structure

Tests use **in-memory SQLite** (`:memory:`) — no external dependencies needed. **15 tests total** across 4 packages.

- `internal/store/sqlite/sqlite_test.go` — Store CRUD tests: `TestProductCRUD`, `TestMachineSlotsAndStatus`, `TestUserCRUD`, `TestDispenseAndRestock`
- `internal/auth/jwt_test.go` — JWT tests: `TestGenerateAndValidateToken`, `TestJWTMiddleware_MissingToken`, `TestJWTMiddleware_InvalidToken`, `TestJWTMiddleware_ValidToken`
- `internal/service/service_test.go` — Service logic tests: `TestLogin` (success + bad password + non-existent user), `TestGetDispenseStats` (verify counts/ranking/cost), `TestGetMachineCost`
- `internal/api/handlers_test.go` — API integration tests using `httptest.NewRecorder()` against the real chi router: `TestHealth`, `TestCreateAndListMachines`, `TestDispenseRequiresAPIKey`, `TestLoginSuccess`, `TestLoginBadPassword`, `TestAdminStatsRequiresAuth`, `TestFullWorkflow` (API key auth), `TestFullWorkflowWithJWT` (JWT auth: login → create product → machine → slots → restock → admin stats → cost → list restocks)

## Key Business Logic

- `internal/service/service.go` — `GetMachineStatus`: computes stock per slot (restock qty - dispenses since restock), assigns status (ok/low/empty), caps stock at 0. `Login`: bcrypt verify + JWT generation. `GetDispenseStats`: in-memory aggregation (hourly bucketing, product ranking, cost). `GetMachineCost`: sum of costPerCan for all dispenses. `ListRestocks`: delegates to store.
- `internal/api/handlers.go` — `createDispense`: auto-resolves productId from slot config if not provided. `login`: authenticates and returns JWT. `getAdminStats`, `getMachineCost`, `listRestocks`: admin analytics endpoints.
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

### Phase 3 — Events & Notifications [NEXT]
- [ ] EventBus interface + local (in-process channel) implementation
- [ ] Alert service: evaluates stock after each dispense, fires low_stock/out_of_stock events
- [ ] Slack bot: push notifications on events, pull commands (`/soda status`, `/soda top`)
- [ ] Discord bot: `discordgo` persistent WebSocket, same push/pull features
- [ ] Alert configuration UI (`/admin/alerts`)
- [ ] Webhook system: CRUD + HMAC-signed delivery
- [ ] Predictive stock-out: linear regression on dispense rate

### Phase 4 — AWS Deployment
- [ ] DynamoDB store implementation (single-table design: PK/SK patterns, one GSI)
- [ ] SNS event bus implementation
- [ ] Lambda entry point (`cmd/lambda/`) using `aws-lambda-go-api-proxy/chi`
- [ ] CDK stack in Go (`infra/cdk/`): DynamoDB, Lambda, API Gateway, SNS, SQS, S3, CloudFront
- [ ] CI/CD via GitHub Actions

### Phase 5 — Polish
- [ ] CSV/PDF export for dispense reports
- [ ] Mobile responsive polish
- [ ] Pi agent systemd service + watchdog
- [ ] Observability: structured logging, CloudWatch metrics
- [ ] E2E tests (Playwright)
