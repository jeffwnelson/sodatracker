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

### Current wiring

The SodaTracker Pi took over the legacy Pi's harness on 2026-09-30. Each column
has an added contact: two bare wires that an arm presses together while the
vend cam turns. Those contacts are separate from the machine's own switch
stack. They run to the Pi over an 8-conductor Ethernet cable, with the
contact commons on two conductors and one conductor spare. The cable's colors
don't match end to end, so the pin table below goes by the Pi-side color.

Every column was verified by closing its contact and by real vends, including
which Diet Mountain Dew column is #1 and which is #2.

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

Each contact connects a GPIO signal pin to ground. The Pi's internal pull-up
holds the pin HIGH while the contact is open, and a vend closes it (LOW). The
bare-wire contacts bounce, so the piagent debounces: a closure counts once the
pin has read LOW for `minClosedMs` (default 100), and the vend is reported,
timestamped at the start of the closure, once the pin has read HIGH again for
`minOpenMs` (default 500). Shorter blips and mid-vend bounces are ignored.

### Pin Assignments

Slots use the machine's own column numbers.

The Pi's header connects to the RJ45 breakout with Dupont jumpers, and the
breakout and the cable's plug are both wired T568B, so breakout position n is
the cable conductor on plug pin n. Settings → Wiring draws the same map.

| Slot | Product           | Pi-side wire | Breakout | Physical | BCM |
|------|-------------------|--------------|----------|----------|-----|
| 1    | Diet Mountain Dew | brown        | 8        | 40       | 21  |
| 2    | Diet Mountain Dew | blue         | 4        | 36       | 16  |
| 3    | Coke Zero         | white/green  | 3        | 35       | 19  |
| 4    | Mountain Dew      | green        | 6        | 38       | 20  |
| 5    | Diet Coke         | white/blue   | 5        | 37       | 26  |
| —    | common            | white/brown  | 7        | 39 (GND) | —   |
| —    | common            | white/orange | 1        | 34 (GND) | —   |
| —    | spare             | orange       | 2        | 33       | 13  |

Every jumper sits on header pins 33–40 and is the same color as the cable
conductor on its breakout position (rewired 2026-10-20).

No external resistors are needed: the piagent enables the internal pull-ups
through periph.io. Nothing should connect to the 3.3V pins (1, 17) or the 5V
pins (2, 4).

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

The compose file publishes port 3000 on the Pi's loopback only, so the
dashboard is reached through `tailscale serve` (below). To publish plain HTTP on
the LAN instead, set `SODATRACKER_BIND=0.0.0.0` in `.env`; the app is then at:

Dashboard: `http://<pi-ip>:3000/`
Admin login: `http://<pi-ip>:3000/admin/login` (user: `admin`, pass: value of `ADMIN_PASSWORD`)
API docs: `http://<pi-ip>:3000/swagger/index.html`

**HTTPS over Tailscale.** With HTTPS certificates enabled in the tailnet's DNS
settings, run this once on the Pi host:

```bash
sudo tailscale serve --bg 3000
```

The app is then at `https://<pi-name>.<tailnet>.ts.net/` with a Let's Encrypt
certificate that Tailscale renews. It persists across reboots. On an iPhone,
open that URL in Safari and use Share → Add to Home Screen.

**Host setup (logs and Wi-Fi watchdog).** The Pi is on Wi-Fi only. When the
link drops or the `brcmfmac` driver wedges, nothing brings it back on its own.
Run this once on the Pi:

```bash
cd ~/sodatracker && sudo deploy/pi/install.sh
```

It installs two things:

- **A persistent journal.** Raspberry Pi OS keeps logs in memory, so a reboot
  erases them. After this, `journalctl -b -1` shows the boot before the last
  one, capped at 200 MB.
- **`wifi-watchdog.timer`.** Every 2 minutes it fetches `https://api.github.com`
  and `https://slack.com`, and any response counts as online. After 5 failed
  checks (about 10 minutes) it reloads the Wi-Fi driver and reconnects
  Wi-Fi (`WATCHDOG_CON`, or whatever network `wlan0` joins), then does that again every 30 minutes while the Pi is still
  offline. After 15 failed checks (about 30 minutes) it reboots the Pi, at most
  once every 6 hours.
  - See what it did: `journalctl -u wifi-watchdog`.
  - Try it without it acting:
    `sudo DRY_RUN=1 WATCHDOG_URLS=https://example.invalid WATCHDOG_FAILS_FILE=/tmp/wd /usr/local/sbin/wifi-watchdog.sh`.
  - Turn it off: `sudo systemctl disable --now wifi-watchdog.timer`.

The app can't send Slack alerts while the Pi is offline, and it doesn't send
them later. So a "Backups recovered" message with no "failed" message before
it usually means the Pi was offline.

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
| `PIAGENT_HEARTBEAT_MS` | `heartbeatMs` |

Pin mappings are YAML-only; there is no environment override for them.

## Backups

The server snapshots the database on a schedule (daily by default), keeps the
last 30 in `/data/backups/`, and, once it's set up, pushes each snapshot to a
private GitHub repo as `snapshots/<year>/sodatracker-<UTC time>.db.gz`. A copy
on the Pi's own SD card doesn't survive the card failing, so set up the push.

**Setup.** Create a private repo (`jeffwnelson/sodatracker-backups`) and a
[fine-grained token](https://github.com/settings/personal-access-tokens/new)
that can reach only that repo, with **Contents: read and write**. Enter both
under **Settings → Backups** and save. The server checks them with GitHub
before saving, then pushes the newest snapshot. The schedule and how many
snapshots to keep are on the same card.

**How it works.** Snapshots use SQLite's `VACUUM INTO`, which gives a
consistent copy while the app keeps running. Copying the file itself would
not, because the database is in WAL mode. The token is stored in the
database but never returned by the API, and it is removed from every snapshot,
so a backup in the repo or a downloaded one doesn't contain it. A failed
snapshot or push, or no push in 36 hours, shows on the Alerts page. Failed
pushes are retried every hour. **Download** on the same card fetches a fresh
snapshot to your browser.

**Restore.** On the Pi, with the snapshot (`.db.gz`) in `~/sodatracker`:

```bash
cd ~/sodatracker
docker compose stop
docker run --rm -v sodatracker-data:/data -v "$PWD":/in \
  --entrypoint sh ghcr.io/jeffwnelson/sodatracker:latest -c '
    set -e; cd /data
    mkdir -p pre-restore && mv sodatracker.db sodatracker.db-* pre-restore/ 2>/dev/null || true
    gunzip -c /in/sodatracker-20261002T030000Z.db.gz > sodatracker.db'
docker compose start
```

This moves the current database and its `-wal`/`-shm` files to
`/data/pre-restore/` instead of deleting them. A WAL file left beside a
restored database would be replayed into it. Then enter the GitHub token again
under Settings → Backups, because snapshots don't carry it.

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
