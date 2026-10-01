# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Goal

Rebuild the existing soda machine tracking prototype (Node.js/AngularJS/MongoDB in `consumables/`) as a modern, production-ready system called **SodaTracker** (in `sodaTracker/`).

**Deployment is self-hosted only.** The system runs as a single Docker container on a
Raspberry Pi. AWS is explicitly out of scope — the Lambda/DynamoDB/SNS implementation
that existed through Phase 4 was removed on 2026-09-24 at the owner's direction,
having never been deployed. Do not reintroduce cloud dependencies.

**Nothing is sold.** The machine is an office soda machine: people vend with
fake 10-cent tokens and pay nothing. The company buys the cans, so cost per
can and the Cost (`spend`) analytics are what the company spends, not revenue.
Say dispense, take or use in UI text and docs, never sale, sell, sold, buy or
price (except the machine's own "sold-out" switch).

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
- **Database:** SQLite via `modernc.org/sqlite` (pure Go, no CGO) — behind a `Store` interface
- **Events:** In-process channel-based pub/sub — behind an `EventBus` interface
- **Frontend:** React 19 + TypeScript + Vite + TanStack Query + Recharts + Tailwind CSS v4 (Vite plugin, no `tailwind.config.js`) + react-router-dom v7
- **Pi agent:** Go binary cross-compiled for ARM; real `periph.io` GPIO behind the `rpi` build tag (mock otherwise), SQLite local cache. Ships inside the same container image as the server.
- **Auth:** JWT (HS256, 24h expiry) for admin dashboard, API key for Pi agent, `APIKeyOrJWT` middleware on shared admin routes
- **IDs:** Server-generated UUIDs via `google/uuid`

## SodaTracker Repository Structure

```
sodaTracker/
├── cmd/
│   ├── server/main.go        # Self-hosted HTTP server
│   ├── piagent/main.go        # Raspberry Pi GPIO agent
│   ├── slackbot/              # Slack bot (Phase 3, empty stub)
│   └── discordbot/            # Discord bot (Phase 3, empty stub)
├── internal/
│   ├── api/                   # chi router, middleware, handlers
│   │   └── docs/              # Generated Swagger/OpenAPI docs (swaggo/swag)
│   ├── domain/                # Entity structs (Product, Machine, Slot, Dispense, Restock, User)
│   ├── store/                 # Store interface + sqlite/ implementation
│   ├── events/                # EventBus interface + local/ implementation
│   ├── service/               # Business logic (dispense, restock, stock calculation, login, stats)
│   ├── seed/                  # Admin user seeding
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
│       │   └── analytics/     # AnalyticsLayout + Consumption/Products/Inventory/Spend/Predictions/Highlights/Uptime dashboards
│       ├── components/        # AppShell (the one page frame: header with version, main), AdminLayout, StockGrid, ProtectedRoute
│       ├── context/           # AuthContext (JWT token + user state, localStorage persistence)
│       └── api/               # API client + TypeScript types
├── .github/workflows/         # CI: one arm64 build (tests inside), cocogitto release, push to GHCR
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

- **SSH:** `ssh pi@sodatracker` (Tailscale, works from any network) or
  `ssh pi@<office-ip>` (office ethernet). Key-based: the owner's
  `~/.ssh/id_ed25519` is authorized. The password is not kept in this repo.
- **GHCR image:** `ghcr.io/jeffwnelson/sodatracker:latest` (linux/arm64 only)
- **Container name:** `sodatracker`
- **Compose dir on Pi:** `/home/pi/sodatracker` (git clone of this repo)
- **Data volume:** `sodatracker-data` at `/data` (SQLite DB + piagent cache)
- **Dashboard URL:** `http://sodatracker:3000` (Tailscale) or `http://<office-ip>:3000` (office ethernet)

**Networking.** The Pi joined the owner's tailnet as `sodatracker`
(`sodatracker`) on 2026-09-24, and `tailscaled` is enabled at boot. Prefer the
Tailscale address: it works from any network, whereas `<office-ip>` only works
from the office ethernet segment. `sodatracker.example.com` no longer resolves.

The office guest WiFi (`office-guest-wifi`, the Pi's fallback link) enforces client
isolation in both directions, so the Pi is unreachable over it by LAN address —
Tailscale is what makes the Pi usable without the ethernet cable. Tailscale was
brought up with `--accept-dns=false` to avoid resolver conflicts with the
corporate network, so there is no MagicDNS name; use the IP.

**Releases (cocogitto).** Commits on `main` must follow
[Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`,
`docs:`, `chore:`, `refactor:`, `test:`, `ci:`, with an optional scope, and `!` or a
`BREAKING CHANGE:` footer for majors). After the image builds and tests pass, CI runs
`cog bump --auto` (config in `cog.toml`): `feat` bumps the minor, `fix` the patch,
and anything else releases nothing. It commits `CHANGELOG.md` as
`chore(version): vX.Y.Z [skip ci]`, pushes the bump commit and tag, and creates
a GitHub release. **Pull before committing after a release**, because `main` then has a
bot commit you don't. History before `v0.1.0` isn't conventional, and
`from_latest_tag = true` keeps cog from checking it.

**Version in the app.** `internal/buildinfo.Version` is stamped by ldflags: the
Dockerfile takes `--build-arg VERSION` (CI passes the version `cog bump --auto
--dry-run` predicts, or `git describe --tags` (`vX.Y.Z-N-gSHA`) when nothing will release), and the Makefile
uses `git describe --tags --always --dirty`. Unstamped builds say `dev`.
`GET /api/v1/health` returns `{"status","version"}`, and `AppVersion` (in the header next to the
brand, on every page) displays it. Release images also get a `vX.Y.Z` image tag.

**CI publishes the image.** `.github/workflows/docker-publish.yml` is one job that
builds once: predict the version (`cog bump --auto --dry-run`), build and push the
**arm64** image as `sha-<commit>` (the Dockerfile's `api-build` stage runs
`go vet` and `go test`, so a failing test fails the build), then do the real bump,
tag and GitHub release, and finally re-tag that same image as `latest` and `vX.Y.Z` with
`docker buildx imagetools create` rather than rebuilding it. Go and npm caches are BuildKit
cache mounts, which persist while the runner pod lives. A push of a `v*` tag builds and
tags that version without moving `latest`. The image is arm64 only; on an Intel
machine use `make run` or `docker compose up --build`.

It runs on the owner's **self-hosted** `sodatracker-runner` (a Docker-in-Docker
StatefulSet defined in the `gitops-deployments` repo, `apps/github-runners`), not on
GitHub-hosted runners. Keep this repo **private** while it uses them. The runners'
Talos kernel has no `binfmt_misc`, so QEMU is unavailable: every `RUN` in the
Dockerfile must stay in a `--platform=$BUILDPLATFORM` stage, and the final stage
only `COPY`s (tini comes from a build-platform stage via `apk --arch`).

```bash
make pi-deploy      # ssh to Pi, docker compose pull && docker compose up -d
make pi-logs        # tail container logs
make docker-push    # manual arm64 build+push, bypassing CI
```

GHCR auth on the Pi is already configured (`~pi/.docker/config.json`). If it expires:
```bash
gh auth token | ssh pi@<office-ip> 'cat | docker login ghcr.io -u jeffwnelson --password-stdin'
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
periph.io GPIO, not the mock). That tag compiles cleanly on amd64 too, so a
local `docker compose up --build` works on an Intel host; it just finds no pins.

**Pi agent config** is mounted at `/config/piagent.yaml` (`PIAGENT_CONFIG` env var).
Pin mappings are YAML-only. Deployment-specific fields have env overrides
(`applyEnvOverrides` in `pkg/piagent/config.go`): `PIAGENT_API_URL`,
`PIAGENT_API_KEY`, `PIAGENT_MACHINE_ID`, `PIAGENT_DB_PATH`, `PIAGENT_POLL_MS`,
`PIAGENT_RETRY_MS`, `PIAGENT_HEARTBEAT_MS`, `PIAGENT_MIN_CLOSED_MS`, `PIAGENT_MIN_OPEN_MS`.

## Key Configuration

- **API port:** 3000 (env `PORT`)
- **SQLite DB path:** `sodatracker.db` (env `DB_PATH`) — opens with `?_journal_mode=WAL&_busy_timeout=5000`
- **Secrets:** the values below are **local-dev defaults only**. The Pi sets real ones in `/home/pi/sodatracker/.env` (rotated 2026-09-30; never commit them — this repo is public).
- **API key:** `dev-api-key` (env `API_KEY`) — used by Pi agent (compose passes it as `PIAGENT_API_KEY`); also accepted on admin CRUD routes via `APIKeyOrJWT` middleware
- **JWT secret:** `dev-jwt-secret` (env `JWT_SECRET`) — signs HS256 tokens for admin dashboard auth
- **Admin password:** `dev-admin-password` (env `ADMIN_PASSWORD`) — bcrypt-hashed only when the `admin` user is first seeded. Changing `ADMIN_PASSWORD` later has no effect until that user is deleted and re-seeded
- **Static file dir:** empty in dev, `/app/static` in Docker (env `STATIC_DIR`) — serves SPA with index.html fallback
- **Frontend dev server:** port 4000 with proxy to API at :3000
- **Log format:** `text` for local dev, `json` for Docker (env `LOG_FORMAT`, default `"text"`)
- **Pi agent config:** YAML at `/config/piagent.yaml` (env `PIAGENT_CONFIG`) — see `piagent.yaml.example`. Pin mappings are YAML-only; other fields accept `PIAGENT_*` env overrides
- **Pi agent cache DB:** `piagent-cache.db` (default, configurable in YAML)

## Data Model

**Domain entities:** Product (upc, name, costPerCan, imageUrl, color — `#rrggbb` or empty, validated and lowercased by `createProduct`), Machine (name, location, slotCount), Slot (machineId, position, productId, gpioPin, capacity, lowThreshold), Dispense (machineId, productId, slotPosition, sourceType, timestamp), Restock (machineId, userId, slots[], timestamp), User (username, passwordHash `json:"-"`, role)

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
| POST | `/api/v1/agents/heartbeat` | API Key | Pi agent liveness (every 30s) | Done |
| GET | `/api/v1/events` | API Key or JWT | Live event stream (SSE) | Done |
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
| GET | `/api/v1/analytics/consumption` | API Key or JWT | Volume, trend, weekday×hour heatmap | Done |
| GET | `/api/v1/analytics/products` | API Key or JWT | Ranking, mix over time, time of day | Done |
| GET | `/api/v1/analytics/inventory` | API Key or JWT | Stock levels over time, time empty, used before restock | Done |
| GET | `/api/v1/analytics/spend` | API Key or JWT | Spend over time, by product, projection | Done |
| GET | `/api/v1/analytics/highlights` | API Key or JWT | First/last can, records, streaks, first-can product | Done |
| GET | `/api/v1/analytics/uptime` | API Key or JWT | Pi agent online/offline hours, outages | Done |
| CRUD | `/api/v1/webhooks` | JWT | Webhook subscriptions | Phase 3 |

**Auth:** API key middleware accepts both `X-API-Key: <key>` and `Authorization: Bearer <key>` headers. The `APIKeyOrJWT` middleware on admin CRUD routes accepts either an API key or a valid JWT token, so both the Pi agent and the admin dashboard can call them.

## Architecture Details

- **JWT auth** uses HS256 signing with 24h token expiry. Claims include `userId`, `username`, `role`. Tokens are generated by `auth.GenerateToken()`, validated by `JWTMiddleware`. The `APIKeyOrJWTMiddleware` tries API key first (X-API-Key header), then JWT from Authorization Bearer header (`internal/auth/jwt.go`).
- **Admin user seeding** — On startup, if no "admin" user exists, one is created with bcrypt-hashed password from `ADMIN_PASSWORD` env var (`internal/seed/`, called by `cmd/server/main.go`).
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
- **EventBus** — `events.EventBus` interface (`internal/events/events.go`) with `Publish`, `Subscribe`, `Close`. Local implementation (`internal/events/local/`) uses synchronous in-process dispatch with `sync.RWMutex`. Event data uses `map[string]any` to stay generic for future webhook delivery. Bus is nil-safe — service skips publish if bus is nil.
- **Alert evaluation flow** — `RegisterDispense` → publishes `EventDispense` → subscriber calls `EvaluateStockAlert` → computes stock → checks alert configs (slot-specific > machine-wide `slotPosition=0` > slot's own `lowThreshold`) → creates `AlertEvent` if threshold breached and no active alert exists (deduplication via `GetActiveAlert`). `CreateRestock` clears active alerts for restocked slots.
- **AlertConfig `slotPosition=0`** means machine-wide default. `LowThreshold=-1` means use the slot's own threshold. Stored with `UNIQUE(machine_id, slot_position)` constraint (upsert on create).
- **Predictive stock-out** — `GetStockPredictions` computes `dispensesPerDay = count / lookbackDays` (default 14 days) per slot, then `estimatedDaysLeft = currentStock / dispensesPerDay`. Returns -1 if no dispense data.
- **Frontend token storage** — localStorage with key `soda_token`. AuthContext initializes from localStorage so page refresh preserves login state.
- **Frontend refresh intervals:** machine list refreshes every 30s, individual machine status cards every 10s, admin stats staleTime is 30s, TanStack Query default staleTime is 5s.
- **TypeScript strict mode** enabled with `verbatimModuleSyntax` — all type imports must use `import type`.
- **Structured logging** — All Go code uses `log/slog` (no `log` package). `internal/logging/Setup()` configures JSON or text handler based on `LOG_FORMAT` env var (default `"text"`, Docker sets `"json"`). Custom request logging middleware replaces chi's `middleware.Logger`, logs method/path/status/duration/requestId via slog. `middleware.RequestID` generates unique request IDs for tracing.
- **Dispense report export** — `GET /api/v1/reports/dispenses?machineId=X&format=csv|pdf` exports dispense history. CSV uses stdlib `encoding/csv`. PDF uses `go-pdf/fpdf` (pure Go, no CGO). Both enrich dispenses with product name/UPC/cost from a product lookup map. PDF includes title, date range, table, and summary footer.
- **Analytics dashboards** — `/admin/analytics/{consumption,products,inventory,spend}` in the web app, backed by `internal/service/analytics.go` and `internal/api/analytics.go`. All four endpoints take `machineId` (required), `from`/`to` (RFC3339; default last 30 days, `to` capped at now), `tz` (IANA name, default UTC) and `bucket` (`day|week|month`, default chosen from window length: ≤62d day, ≤366d week, else month). Each request loads the machine's **full** dispense and restock history and aggregates in Go (same in-memory approach as `GetDispenseStats`). `from` is clamped to the start of the day tracking began, so averages aren't diluted by pre-tracking days; the response's `range` echoes the effective window. Previous-period comparisons (`previousTotal`, `previousRank`) are `null` unless tracking covers the whole preceding window of equal length. Inventory replays restocks (set level) and dispenses (−1, floored at 0) per slot — the same model as `GetMachineStatus` — and reports dispenses recorded at computed zero stock as `untrackedDispenses`. Buckets and hours are computed in `tz`; `cmd/server` imports `time/tzdata` because the Alpine image has no zoneinfo. Every response's `range.trackingSince` is the machine's first dispense or restock ever (null with neither), even when the window is wholly before it. Frontend keeps machine, range and comparison in the URL query so tabs share them: `?machine=&range=<7d|30d|90d|12m|all|custom>&from=&to=` (custom whole local days, inclusive) and `&compare=<previous|year|custom>&cfrom=&cto=`. The range picker (`DateRangePicker.tsx`, after Countly's) has presets, a two-month calendar and a compare mode; the selected range is accent and the comparison orange (`COMPARE_COLOR`). Comparison is client-side: `useComparison` fetches the same endpoint for the comparison window with the primary response's `bucket`, aligns series by bucket offset from each range's first day, and returns null when the comparison is wholly before tracking (`ComparisonNotice` explains, and also flags partly covered ones). A comparison of a range ending today stops at the same time of day. Stat tiles show it via `StatTile compare` + `Delta`; Consumption and Cost overlay it as a dashed line; Products adds comparison count and rank change. The API's own `previousTotal`/`previousRank` are no longer used by the UI. and uses a fixed, CVD-validated categorical palette in `web/src/pages/analytics/analytics.ts` with colors keyed by product ID (not rank); `productColorMap` lets a product's own `color` (set on the Setup page, also shown as a `ProductDot` in the stock grid and machine detail) override its palette slot. Every chart has a table view.
- **Live updates (SSE)** — `GET /api/v1/events` (`internal/api/stream.go`) streams bus events (`dispense`, `low_stock`, `out_of_stock`, `stock_cleared`, `agent_online`) as Server-Sent Events, behind `APIKeyOrJWT`. A `broker` subscribes to the bus once and fans out to per-connection buffered channels, dropping rather than blocking, because the bus calls subscribers synchronously from the publishing request. The handler clears the server's 15s `WriteTimeout` with `http.ResponseController`, sends a keepalive comment every 20s, ends a JWT stream when the token expires, and ends all streams when `RouterConfig.StreamsDone` closes (`cmd/server` closes it via `RegisterOnShutdown`, or `Shutdown` would wait on them). The browser reads it with `fetch` (`streamEvents` in `web/src/api/client.ts`), not `EventSource`, which can't send the Authorization header. `LiveUpdatesProvider` (`web/src/context/LiveUpdates.tsx`) holds one stream while logged in, reconnects with backoff, invalidates every query whose key contains the event's `machineId`, and refetches everything after a reconnect. `usePollInterval(ms)` slows polling to 60s while the stream is up; logged-out visitors poll as before. The header shows Live/Reconnecting.
- **Pi agent heartbeat** — the agent POSTs `/agents/heartbeat` every `heartbeatMs` (30s) from its own goroutine. `internal/service/agents.go` stores beats as `agent_sessions` rows (an unbroken run of beats, `started_at`..`last_seen`, updated each beat) and starts a new session, publishing `agent_online`, when a beat comes more than `AgentOfflineAfter` (90s) after the last. `GET /machines/:id/status` includes `agent: {online, lastSeen}`, shown by `AgentStatusBadge` on the public page and machine detail. There's no offline event: pages notice through the 60s backstop poll.
- **Highlights and uptime analytics** — `/admin/analytics/highlights` and `/uptime`, same query parameters as the other four. Highlights (`analytics_highlights.go`): average and record first/last can of the day, busiest day and clock hour, longest gap between dispenses, weekday streaks (weekends neither count nor break; today doesn't break the current streak until its first dispense), and which product is each day's first can (pie). Uptime (`analytics_uptime.go`) replays `agent_sessions`: a session is online from first to last beat, the last one runs to `to` while its beat is recent, the gaps are outages, and time before the first-ever beat is untracked. The pie shows only tracked time; the stacked bars include untracked. Heartbeats began with v0.4.0 (2026-10-01), so earlier time is untracked.
- **Swagger/OpenAPI** — Generated by `swaggo/swag` v2 from Go comment annotations. `internal/api/router.go` has the general API info block (title, version, security definitions). Each handler in `handlers.go` has `@Summary`, `@Description`, `@Tags`, `@Param`, `@Success`/`@Failure`, `@Security`, and `@Router` annotations. `internal/api/dto.go` defines named response types (`ErrorResponse`, `HealthResponse`, `LoginRequest`, `LoginResponse`, `DeleteResponse`, `CostResponse`) for swag reference. Domain and service types have `example` struct tags. Generated docs (`internal/api/docs/`) are committed. Swagger UI served at `/swagger/*` via `swaggo/http-swagger/v2`. Regenerate with `make swagger`.

## Test Structure

Tests use **in-memory SQLite** (`:memory:`) — no external dependencies needed. **69 tests total** across 9 packages, all run by `go test ./...` with no build tags or external services.

- `internal/events/local/local_test.go` — EventBus tests (5): `TestPublishSubscribe`, `TestMultipleSubscribers`, `TestUnsubscribe`, `TestPublishNoSubscribers`, `TestDifferentEventTypes`
- `internal/store/sqlite/sqlite_test.go` — Store CRUD tests (6): `TestProductCRUD`, `TestMachineSlotsAndStatus`, `TestUserCRUD`, `TestAlertConfigCRUD`, `TestAlertEventCRUD`, `TestDispenseAndRestock`
- `internal/auth/jwt_test.go` — JWT tests (4): `TestGenerateAndValidateToken`, `TestJWTMiddleware_MissingToken`, `TestJWTMiddleware_InvalidToken`, `TestJWTMiddleware_ValidToken`
- `internal/service/service_test.go` — Service logic tests (6): `TestLogin` (success + bad password + non-existent user), `TestGetDispenseStats`, `TestGetMachineCost`, `TestEvaluateStockAlert`, `TestCreateRestockClearsAlerts`, `TestGetStockPredictions`
- `internal/api/handlers_test.go` — API integration tests (17) using `httptest.NewRecorder()` against the real chi router with EventBus wired: `TestHealth`, `TestCreateAndListMachines`, `TestDispenseRequiresAPIKey`, `TestLoginSuccess`, `TestLoginBadPassword`, `TestAdminStatsRequiresAuth`, `TestFullWorkflow`, `TestFullWorkflowWithJWT`, `TestAlertConfigCRUD`, `TestPredictionsEndpoint`, `TestFullWorkflowWithAlerts`, `TestExportDispensesCSV`, `TestExportDispensesPDF`, `TestDeleteProduct`, `TestDeleteProductInUse`, `TestDeleteMachine`, `TestExportRequiresAuth`
- `internal/api/stream_test.go` — Event stream auth, delivery past `WriteTimeout`, close on shutdown; heartbeat endpoint (2)
- `internal/service/agents_test.go` — Online/offline transitions, `agent_online` dedup, status survives restart (1)
- `internal/export/export_test.go` — Export unit tests (3): `TestWriteDispensesCSV`, `TestWriteDispensesPDF`, `TestEmptyExport`
- `internal/service/analytics_test.go` — Analytics tests (5): `TestConsumptionAnalytics` (includes a DST-spanning window), `TestAnalyticsClampsToTrackingStart`, `TestProductAnalytics`, `TestInventoryAnalytics`, `TestSpendAnalytics`
- `internal/service/analytics_health_test.go` — `TestUptimeAnalytics`, `TestHighlightsAnalytics` (2)
- `internal/api/analytics_test.go` — Analytics endpoint auth, 404 and parameter validation (1): `TestAnalyticsEndpoints`
- `internal/api/slack_test.go` — Slack slash command signature checks (5)
- `internal/notify/slack_test.go` — Slack notifier (3)
- `pkg/piagent/debounce_test.go`, `agent_test.go` — Debounce against recorded bounce patterns, one dispense per bouncy vend end to end, and heartbeats (5)

## Key Business Logic

- `internal/service/service.go` — `GetMachineStatus`: computes stock per slot (restock qty - dispenses since restock), assigns status (ok/low/empty), caps stock at 0. `Login`: bcrypt verify + JWT generation. `GetDispenseStats`: in-memory aggregation (hourly bucketing, product ranking, cost). `GetMachineCost`: sum of costPerCan for all dispenses. `RegisterDispense`: store + publish EventDispense. `CreateRestock`: store + clear alerts + publish EventStockCleared. `EvaluateStockAlert`: compute stock, resolve threshold from configs, deduplicate, create AlertEvent. `GetStockPredictions`: dispenses/day over lookback window, estimate days until stock-out.
- `internal/api/handlers.go` — `createDispense`: auto-resolves productId from slot config if not provided. `login`: authenticates and returns JWT. `getAdminStats`, `getMachineCost`, `listRestocks`: admin analytics. Alert CRUD: `listAlertConfigs`, `createAlertConfig`, `updateAlertConfig`, `deleteAlertConfig`. `listAlertEvents`: fired alerts with `?active=true` filter. `getStockPredictions`: per-slot predictions with `?lookbackDays` param. `exportDispenses`: CSV/PDF report with `?machineId&format=csv|pdf` params.
- `pkg/piagent/agent.go` — polls GPIO pins every 100ms and debounces each through `debounce.go`: a closure (LOW) counts after `minClosedMs` (100) and is reported after the pin reads open (HIGH) for `minOpenMs` (500), timestamped at the closure start, so a bouncing bare-wire contact counts once per vend. A contact already closed at startup is not reported. Sends to API, caches failures in local SQLite for retry every 30s (max 50 pending events per retry cycle, stops on first failure)
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
- [x] Alerts page (`/admin/alerts`): active alerts. The alert-config UI was removed 2026-09-30 (the API remains); with no configs, each slot's Low Threshold from Setup applies
- [x] Predictive stock-out: dispense rate over a 14-day lookback, shown on the Analytics → Predictions tab + machine detail page
- [x] Tests: 32 total — EventBus (5), store (6), auth (4), service (6), API (11)
- [ ] Slack bot: push notifications on events, pull commands (`/soda status`, `/soda top`)
- [ ] Discord bot: `discordgo` persistent WebSocket, same push/pull features
- [ ] Webhook system: CRUD + HMAC-signed delivery

### Phase 4 — AWS Deployment [REMOVED]
Built in commit `943e9b5`, never deployed, and deleted on 2026-09-24 at the owner's
direction: DynamoDB store, SNS publisher, Lambda entry point, OpenTofu infrastructure,
and the AWS SDK dependencies. SodaTracker is self-hosted only. See the Project Goal
section — do not reintroduce this.

### Phase 5 — Polish [IN PROGRESS]
- [x] Structured logging: `log/slog` replaces `log` across all Go code, `LOG_FORMAT` env var (`text`/`json`)
- [x] Request ID middleware (`middleware.RequestID`) + custom structured request logging
- [x] CSV/PDF export for dispense reports (`GET /api/v1/reports/dispenses?format=csv|pdf`)
- [x] Swagger/OpenAPI docs via `swaggo/swag` — interactive UI at `/swagger/index.html`, all endpoints annotated
- [x] Analytics dashboards: consumption, products, inventory, spend (`/admin/analytics`)
- [ ] Mobile responsive polish
- [x] Pi agent runs in-container (no systemd unit needed; Docker restart policy is the watchdog)
- [ ] CloudWatch custom metrics
- [ ] E2E tests (Playwright)
