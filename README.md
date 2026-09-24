# SodaTracker

A soda machine tracking system that monitors dispenses via GPIO-connected contact switches on a Raspberry Pi, tracks inventory in real time, and provides an admin dashboard for analytics and restocking.

## Architecture

```
                    ┌──────────────────────────────────────────┐
  Raspberry Pi      │  Docker Container                        │
                    │                                          │
  5 contact  ──────▶│  /dev/gpiomem ──▶ piagent ──▶ API server │
  switches   GPIO   │                    (HTTP on localhost)   │
  on GPIO pins      │                    + React dashboard     │
                    │                    + SQLite (/data/)     │
                    └──────────────────────────────────────────┘
                                    :3000
```

Both processes run in a single container published to GHCR:

- **piagent** polls the GPIO pins every 100ms and POSTs dispense events to the API over localhost. It reads real pins through periph.io, which needs `/dev/gpiomem` mapped into the container.
- **API server** serves the REST API and the React dashboard on port 3000, backed by SQLite in the `/data` volume.

The entrypoint takes a mode argument: `all` (both processes, the default), `server` (API only, for hosts without GPIO), or `piagent` (agent only, pointed at a remote API via `PIAGENT_API_URL`).

## GPIO Wiring

Each contact switch connects a GPIO signal pin to a ground pin. The Pi's internal pull-up resistor holds the pin HIGH when the switch is open. Closing the switch pulls it LOW; the piagent registers a dispense on the release (LOW → HIGH transition).

### Pin Assignments

| Slot | Product              | Physical Signal | Physical GND | BCM Pin |
|------|----------------------|-----------------|--------------|---------|
| 1    | Coca-Cola            | 13              | 14           | 27      |
| 2    | Diet Mountain Dew #1 | 29              | 30           | 5       |
| 3    | Diet Mountain Dew #2 | 33              | 34           | 13      |
| 4    | Mountain Dew         | 37              | 39           | 26      |
| 5    | Diet Coke            | 11              | 9            | 17      |

Each switch is normally-open, wired between its signal pin and an adjacent ground pin. No external resistors needed — the Pi's internal pull-ups are enabled by the piagent via periph.io.

## Quick Start

### On the Raspberry Pi (compose, recommended)

```bash
cd ~/sodatracker
git pull
cp .env.example .env          # first time only, then edit secrets
cp piagent.yaml.example piagent.yaml   # first time only, then set machineId
docker compose pull
docker compose up -d
```

`docker-compose.yml` already maps `/dev/gpiomem` and `/dev/gpiochip0` into the
container, so GPIO works without any host-side agent.

Dashboard: `http://<pi-ip>:3000/`
Admin login: `http://<pi-ip>:3000/admin/login` (user: `admin`, pass: value of `ADMIN_PASSWORD`)
API docs: `http://<pi-ip>:3000/swagger/index.html`

### Plain `docker run` equivalent

```bash
docker run -d \
  --name sodatracker \
  --restart unless-stopped \
  -p 3000:3000 \
  --device /dev/gpiomem:/dev/gpiomem \
  --device /dev/gpiochip0:/dev/gpiochip0 \
  -v sodatracker-data:/data \
  -v /home/pi/piagent.yaml:/config/piagent.yaml:ro \
  -e API_KEY=dev-api-key \
  -e JWT_SECRET=dev-jwt-secret \
  -e ADMIN_PASSWORD=dev-admin-password \
  -e PIAGENT_API_URL=http://127.0.0.1:3000 \
  -e PIAGENT_API_KEY=dev-api-key \
  -e PIAGENT_DB_PATH=/data/piagent-cache.db \
  ghcr.io/jeffwnelson/sodatracker:latest all
```

### Without GPIO hardware

Append `server` instead of `all` (or set `SODATRACKER_MODE=server` in `.env`)
and drop the `--device` flags. The agent is skipped entirely.

### Pi agent configuration

Pin-to-slot mappings come from the YAML file mounted at `/config/piagent.yaml`
— see `piagent.yaml.example`. Deployment-specific fields can be overridden with
environment variables, which is what the compose file does:

| Variable | Overrides |
|----------|-----------|
| `PIAGENT_API_URL` | `apiUrl` |
| `PIAGENT_API_KEY` | `apiKey` |
| `PIAGENT_MACHINE_ID` | `machineId` |
| `PIAGENT_DB_PATH` | `dbPath` |
| `PIAGENT_POLL_MS` | `pollMs` |
| `PIAGENT_RETRY_MS` | `retryMs` |

Pin mappings are YAML-only; there is no environment override for them.

## Development

All commands from the `sodaTracker/` directory.

```bash
make build          # Build server + piagent binaries
make run            # Build and run API server on :3000
make test           # Run all Go tests
make lint           # Run golangci-lint

make web-install    # npm install (first time)
make web-dev        # Vite dev server on :4000 (proxies /api → :3000)
make web-build      # Production frontend build

make docker-up      # docker compose up --build -d
make docker-down    # docker compose down

make swagger        # Regenerate Swagger/OpenAPI docs
```

### Rebuilding & Deploying the Container

Pushing to `main` runs the tests and publishes a multi-arch image
(`linux/amd64` + `linux/arm64`) to `ghcr.io/jeffwnelson/sodatracker:latest`
via `.github/workflows/docker-publish.yml`. Each build is also tagged with its
commit SHA, and git tags matching `v*` publish that version tag.

To roll it out on the Pi:

```bash
make pi-deploy      # ssh in, docker compose pull && up -d
make pi-logs        # tail the container logs
```

To build and push by hand instead of via CI:

```bash
make docker-push                       # arm64 → :latest
make docker-push TAG=test PLATFORMS=linux/arm64
```

## Tech Stack

- **Backend:** Go, chi router, SQLite (pure Go via modernc.org/sqlite)
- **Frontend:** React 19, TypeScript, Vite, TanStack Query, Recharts, Tailwind CSS v4
- **Pi Agent:** Go, periph.io (GPIO), local SQLite cache for offline resilience
- **Deployment:** Docker, self-hosted on a Raspberry Pi; image published to GHCR by CI
- **Auth:** JWT (admin dashboard), API key (Pi agent)
