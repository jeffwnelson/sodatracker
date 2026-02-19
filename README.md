# SodaTracker

A soda machine tracking system that monitors dispenses via GPIO-connected contact switches on a Raspberry Pi, tracks inventory in real time, and provides an admin dashboard for analytics and restocking.

## Architecture

```
┌──────────────────┐       ┌──────────────────────────────────┐
│  Raspberry Pi    │       │  Docker Container (Pi)           │
│                  │       │                                  │
│  piagent binary  │──────▶│  Go API server (:3000)           │
│  (reads GPIO,    │ HTTP  │  + React dashboard (static)      │
│   sends events)  │       │  + SQLite database (/data/)      │
└──────┬───────────┘       └──────────────────────────────────┘
       │
  5 contact switches
  wired to GPIO pins
```

- **piagent** runs natively on the Pi (needs GPIO access), polls pins every 100ms, and POSTs dispense events to the API
- **SodaTracker API + dashboard** runs in Docker on the same Pi, serving both the REST API and the React frontend on port 3000

## GPIO Wiring

Each contact switch connects a GPIO signal pin to a ground pin. The Pi's internal pull-up resistor holds the pin HIGH when the switch is open. Closing the switch pulls it LOW; the piagent registers a dispense on the release (LOW → HIGH transition).

### Pin Assignments

| Slot | Product              | Physical Signal | Physical GND | BCM Pin |
|------|----------------------|-----------------|--------------|---------|
| 1    | Diet Coke            | 8               | 6            | 14      |
| 2    | Coca-Cola            | 12              | 14           | 18      |
| 3    | Mountain Dew         | 7               | 9            | 4       |
| 4    | Diet Mountain Dew #1 | 23              | 25           | 11      |
| 5    | Diet Mountain Dew #2 | 37              | 39           | 26      |

Each switch is normally-open, wired between its signal pin and an adjacent ground pin. No external resistors needed — the Pi's internal pull-ups are enabled by the piagent via periph.io.

## Quick Start

### Run the API + Dashboard (Docker)

```bash
docker run -d \
  --name sodatracker \
  --restart unless-stopped \
  -p 3000:3000 \
  -v sodatracker-data:/data \
  -e API_KEY=dev-api-key \
  -e JWT_SECRET=dev-jwt-secret \
  -e ADMIN_PASSWORD=dev-admin-password \
  ghcr.io/jeffwnelson/sodatracker:latest
```

Dashboard: `http://<pi-ip>:3000/`
Admin login: `http://<pi-ip>:3000/admin/login` (user: `admin`, pass: value of `ADMIN_PASSWORD`)

### Run the Pi Agent (native)

```bash
# Build on the Pi (with real GPIO support)
cd ~/sodatracker
GOOS=linux GOARCH=arm64 go build -tags rpi -o piagent ./cmd/piagent

# Run (requires root for GPIO access)
sudo ./piagent -config piagent.yaml
```

See `piagent.yaml.example` for config format.

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
```

### Rebuilding & Deploying the Container

```bash
# Build for arm64 and push to GHCR
docker buildx build --platform linux/arm64 -t ghcr.io/jeffwnelson/sodatracker:latest --push .

# On the Pi, pull and restart
docker pull ghcr.io/jeffwnelson/sodatracker:latest
docker stop sodatracker && docker rm sodatracker
docker run -d --name sodatracker --restart unless-stopped \
  -p 3000:3000 -v sodatracker-data:/data \
  -e API_KEY=dev-api-key \
  -e JWT_SECRET=dev-jwt-secret \
  -e ADMIN_PASSWORD=dev-admin-password \
  ghcr.io/jeffwnelson/sodatracker:latest
```

## Tech Stack

- **Backend:** Go, chi router, SQLite (pure Go via modernc.org/sqlite)
- **Frontend:** React 19, TypeScript, Vite, TanStack Query, Recharts, Tailwind CSS v4
- **Pi Agent:** Go, periph.io (GPIO), local SQLite cache for offline resilience
- **Deployment:** Docker (self-hosted), also supports AWS Lambda + DynamoDB
- **Auth:** JWT (admin dashboard), API key (Pi agent)
