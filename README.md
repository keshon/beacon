
# Beacon

**Lightweight self-hosted uptime monitor for HTTP and TCP endpoints**

Beacon is a lightweight uptime monitoring tool for HTTP and TCP services.

It runs as a single Go binary, periodically checks your targets, sends alerts on failures and recovery, and provides a simple web dashboard with recent uptime history.

No agents, no Prometheus stack, no external dependencies. Point it at a URL or `host:port` and it works.

![Dashboard](static/example.png)

## Why Beacon

| | |
|---|---|
| **Small footprint** | Single binary, minimal setup, lightweight runtime requirements |
| **Simple web UI** | Dashboard, monitor management, live status updates, dark/light theme |
| **Flexible notifications** | Telegram, Discord, email, and webhooks with per-monitor channel overrides |
| **Multi-instance sync (optional)** | Sync monitors and state between multiple Beacon instances |

## When Beacon makes sense

Beacon is designed for:

- small VPS setups
- self-hosted services
- side projects
- homelabs
- lightweight production monitoring
- simple uptime alerting without a full observability stack

If you need metrics storage, distributed tracing, or deep infrastructure analytics, use a full observability platform instead.

## Features

- HTTP and TCP checks with interval, timeout, and retry threshold
- HTTP checks: optional Basic Auth and response keyword matching
- Down and recovery notifications via Telegram, Discord, email, and generic webhooks
- Global notification settings with per-monitor per-channel overrides (Global / Off / Custom)
- Real-time dashboard updates and uptime history
- Basic authentication for web UI and API
- CLI for managing monitors and inspecting state
- REST API for automation and integration

## Quick start

**Requirements:** Go 1.24+ (see `go.mod`)

```bash
git clone https://github.com/keshon/beacon.git
cd beacon
go run ./cmd/beacon
````

Open [http://localhost:8080](http://localhost:8080). Default login is `admin` / `admin`. Change credentials before exposing Beacon to a network.

Data is stored under `./data/`. On first run, if `config.json` is missing, default configuration is written automatically.

### Custom config path

```bash
go run ./cmd/beacon /path/to/config.json
```

### Build binary

```bash
go build -o beacon ./cmd/beacon
./beacon
```

## Configuration

Settings can be managed via the web UI or `config.json`.

| Area     | Notes                                              |
| -------- | -------------------------------------------------- |
| Server   | Listen address, worker pool size, default interval |
| Auth     | HTTP basic auth for UI and API                     |
| Telegram | Up to 5 bot token + chat ID pairs                  |
| Discord  | Up to 5 webhook URLs                               |
| Email    | Global SMTP + up to 5 recipient addresses          |
| Webhook  | Up to 5 generic HTTP POST endpoints                |
| Sync     | Multi-instance monitor/state synchronization       |

Legacy configuration formats are automatically migrated.

Example minimal `config.json`:

```json
{
  "listen": ":8080",
  "workers": 10,
  "default_interval": 30,
  "auth": {
    "username": "admin",
    "password": "change-me"
  },
  "telegram": {
    "enabled": true,
    "targets": [
      { "token": "YOUR_BOT_TOKEN", "chat_id": "YOUR_CHAT_ID" }
    ]
  },
  "discord": {
    "enabled": false,
    "webhooks": [
      { "webhook": "https://discord.com/api/webhooks/..." }
    ]
  },
  "email": {
    "enabled": false,
    "smtp": {
      "host": "smtp.example.com",
      "port": 587,
      "username": "user",
      "password": "secret",
      "from": "beacon@example.com",
      "tls": "starttls"
    },
    "targets": [
      { "to": "ops@example.com" }
    ]
  },
  "webhook": {
    "enabled": false,
    "webhooks": [
      { "url": "https://hooks.example.com/beacon" }
    ]
  },
  "notifications": {
    "alert_mode": "repeat",
    "templates": {
      "down": "Service DOWN\\n\\n{{name}}\\n{{message}}\\nTime: {{time}}",
      "recovered": "Service RECOVERED\\n\\n{{name}}\\n{{message}}\\nTime: {{time}}"
    }
  },
  "network": {
    "enabled": false
  }
}
```

## Notifications

### Global defaults (Settings → Notifications)

Set the default **alert mode** and **message templates** for all receivers. Empty fields in a receiver’s policy (gear icon on each row) inherit these values.

| Mode | Behavior |
|------|----------|
| **Repeat while down** (default) | Sends an alert on every failed check while the monitor is down |
| **Once on down + recovery** | One alert when the monitor goes down, one when it recovers |

### Per-receiver policy

Each Telegram, Discord, and webhook receiver row has a **gear** button to configure:

- Alert mode (`repeat` / `once`) for that destination only
- Custom **down** and **recovered** templates (per field; empty = use global default)

Row badges show the effective mode (**Repeat** / **Once**) and whether templates are **Standard** (built-in) or **Custom** (differs from built-in defaults).

Different receivers on the same monitor can use different modes (for example, ops channel `repeat`, management `once`).

### Message templates

Customize plain-text bodies for **down** and **recovered** using `{{placeholders}}`. In the receiver policy modal (gear icon), use **Test** next to each template field to send a preview with sample placeholder values to that receiver (works before Save).

| Placeholder | Value |
|-------------|--------|
| `name` | Monitor name |
| `target` | URL or host:port |
| `type` | `http` or `tcp` |
| `status` | `down`, `recovered`, or `test` |
| `error` | Check error text |
| `latency` | Response latency |
| `status_code` | HTTP status code (`0` if N/A) |
| `time` | Event time |
| `message` | Detail line (error or latency summary) |
| `fail_count` | Failed checks before marking down |

Use **Reset** on a field (or **Reset all**) to restore built-in defaults. `GET /api/notify/defaults` returns built-in templates and the placeholder list for the UI.

### Global receivers

Configure Telegram, Discord, email, and webhook receivers in Settings (up to 5 per channel). Each row supports Test, remove, and per-receiver policy. Email alerts are always sent once on down and once on recovery (no repeat mode).

### Per-monitor overrides

For each channel (Telegram, Discord, email, webhook), choose **Global** (follow Settings), **Off** (disable for this monitor), or **Custom** (monitor-specific receiver list). Custom lists replace global receivers for that channel only.

Legacy flat `notify_override` arrays (e.g. `{ "telegram": [...] }`) are migrated to **Custom** mode on load.

### HTTP monitor options

For HTTP monitors, expand **HTTP options** to set Basic Auth credentials (stored server-side; password is never shown in the UI after save) and an optional response **keyword** (must appear in the body, or enable **Must not contain** to invert).

## Multi-instance sync

Beacon supports optional synchronization between multiple instances.

When enabled, instances exchange monitor definitions and state.

This is useful for running multiple Beacon nodes in parallel environments.

Configuration:

* self URL
* peer list
* sync interval
* timeout settings

Export endpoint:

```
GET /api/sync/export
```

Requires sync to be enabled.

## Web UI

| Route        | Purpose                          |
| ------------ | -------------------------------- |
| `/dashboard` | Status overview and live updates |
| `/monitors`  | Manage monitors                  |
| `/settings`  | Configuration and sync           |
| `/login`     | Authentication                   |

## CLI

```bash
# Monitors
beacon monitor list
beacon monitor add -name "API" -type http -target https://api.example.com
beacon monitor add -name "Redis" -type tcp -target redis.internal:6379
beacon monitor delete <id>
beacon monitor update <id>

# State
beacon state
beacon events -limit 100
```

CLI uses the same datastore as the server.

**Target format:** HTTP monitors need a full URL (`http://` or `https://`). TCP monitors need `host:port` with no scheme (e.g. `db.local:5432`). The API and UI validate targets on create/update.

## HTTP API

All endpoints require authentication (HTTP Basic or session cookie). Cookie-authenticated `POST`/`PUT`/`PATCH`/`DELETE` requests must include the `X-CSRF-Token` header matching the `beacon_csrf` cookie (see `static/beacon.js`).

| Method | Path                      | Description            |
| ------ | ------------------------- | ---------------------- |
| GET    | /api/health               | Health check           |
| GET    | /api/monitors             | List monitors          |
| POST   | /api/monitors             | Create monitor         |
| PATCH  | /api/monitors/{id}        | Update monitor         |
| DELETE | /api/monitors/{id}        | Delete monitor         |
| GET    | /api/monitors/{id}/uptime | Uptime samples         |
| GET    | /api/state                | Current state          |
| GET    | /api/check-records        | Check history records  |
| GET    | /api/config               | Get config             |
| PUT    | /api/config               | Update config          |
| POST   | /api/notify/test          | Send test notification |
| GET    | /api/notify/defaults      | Default alert mode, templates, placeholders |
| GET    | /api/stream/checks        | Live check stream      |
| GET    | /api/sync/export          | Sync export            |
| GET    | /api/network/status       | Sync status            |

## Docker

Docker support is provided via a multi-stage build in `docker/`.

Build example:

```bash
docker build -f docker/Dockerfile -t beacon .
```

Docker Compose example is available in `docker/docker-compose.yml`.

## Development

* Templates: `templates/`
* Styles: `uikit/scss/`
* Static assets: `static/`

Run tests:

```bash
go test ./...
```

## Project layout

```
cmd/beacon/              Entry point
internal/
  checks/                HTTP and TCP probes
  commands/              CLI commands (commandkit)
  config/                Configuration, AuthCredentials, receivers
  monitor/               Monitors, StatusEvaluator, validation
  netpolicy/             SSRF / host allowlist for probes
  notify/                Telegram/Discord delivery
  scheduler/             Check scheduling
  service/               Shared domain logic (monitors, config, state)
  sse/                   CheckStreamHub for live check SSE
  store/                 Persistence (CheckRecord history)
  sync/                  PeerSyncClient for multi-instance sync
  web/                   Pages (page*) and JSON API (api*)
templates/
  dashboard/             Dashboard page and row partials
  monitors/              Monitors page and form partials
  settings/              Settings page
  partials/              Shared head fragments
  base.html, login.html  Root layouts
static/                  beacon.js (Beacon.apiFetch, CSRF) + notify UI
tooling/scripts/         UIKit bootstrap/build/watch helpers
docs/REMAINING.md        Optional follow-up backlog
```

## License

MIT License

Copyright (c) 2026 Innokentiy Sokolov
