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

## Hardware

The machine is most likely a **Dixie-Narco DNCB 180/105-5**, a small 1980s
Coca-Cola can vender. The model comes from matching its appearance and interior
against auction listings, not from its serial plate, so it is unconfirmed until
someone reads the plate on the right side of the cabinet.

Specs for that model, from the [Dixie-Narco DNCB service manual][dn-manual]
(February 1989):

- **Capacity:** 180 twelve-ounce cans (or 105 regular bottles) across 5
  double-depth columns. Each slot is configured with a capacity of 36, which
  assumes the columns hold equal shares; that split isn't in the manual.
- **Size:** 56-11/16" high, 28-5/16" wide, 26" deep; 462 lb shipping weight.
- **Power:** 115V single phase, 60Hz, never below 103V or above 127V.

Columns are labelled 1–5 from right to left when facing the open door. Each
has its own vend motor turning a cam, with three stacked microswitches beside
it: the vend motor switch, cam-hold switch and by-pass switch, which together
run the motor for exactly one cam revolution per vend. A sold-out switch at the
base of each stack is held down by the cans. Column 2's cam and motor don't
match the others and look like a replacement.

### How a vend turns the cam

Each vend turns that column's cam exactly one revolution. At rest, the cam's
notch sits under the switch arms. A coin arms the circuit, the select button
starts the motor, the arms ride up onto the cam's high side while the can is
released, and after one revolution they drop back into the notch and the vend
motor switch stops the motor. A switch that follows the cam therefore closes
once and reopens once per real vend; the piagent counts the reopening
(LOW → HIGH). Button presses with no coin or on a sold-out column don't turn
the cam; a jam, where the cam turns but no can falls, would still count.

### Current wiring status (unverified)

- The **legacy Pi (Model 2 B)** is wired to the cams over an Ethernet cable.
  Something at the cams is covered with duct tape, so what those conductors
  actually connect to is not known.
- The **SodaTracker Pi** has only been tested with bench contact buttons. The
  pin assignments below come from that bench setup, not from the cam wiring,
  and dispenses recorded so far are likely test presses.

**Mains voltage:** the machine's entire vend control circuit runs at 115V AC.
The service manual draws it "across the line" between L1 and L2, so every
select, cam-hold, by-pass, vend motor and sold-out switch, and every relay, is
live. A GPIO pin must never touch any of those contacts. Before connecting the
SodaTracker Pi to the cam wiring: unplug the machine and inspect under the
tape; confirm each conductor goes only to its own isolated switch, with no
continuity to any machine switch terminal or harness wire; then, with the
machine powered and the legacy Pi disconnected, measure AC and DC from each
conductor to the cabinet at rest and during a vend. Connect one channel first.

[dn-manual]: https://www.vend-resource.com/sites/default/files/dixie-narco-pre-series-90-service-manual.pdf

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
