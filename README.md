
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
| **Flexible notifications** | Global Telegram and Discord receivers plus per-monitor overrides |
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
- Down and recovery notifications via Telegram and Discord
- Global notification settings with per-monitor overrides
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
    "webhooks": []
  },
  "network": {
    "enabled": false
  }
}
```

## Notifications

### Global receivers

Configure Telegram and Discord receivers in Settings (up to 5 per channel).

Each receiver can be tested individually before saving configuration.

### Per-monitor overrides

Each monitor can override notification routing for a specific channel.

If a channel override is set, it replaces global receivers for that channel only.

If left empty, global settings are used.

Example:

* Global Telegram receivers notify multiple chats
* One critical monitor overrides Telegram to only notify a single on-call chat

### Alert content

Alerts include:

* monitor name
* status (down or recovered)
* message
* timestamp

Test notifications are clearly marked as test messages.

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
beacon monitor delete <id>
beacon monitor update <id>

# State
beacon state
beacon events -limit 100
```

CLI uses the same datastore as the server.

## HTTP API

All endpoints require basic authentication unless otherwise noted.

| Method | Path                      | Description            |
| ------ | ------------------------- | ---------------------- |
| GET    | /api/health               | Health check           |
| GET    | /api/monitors             | List monitors          |
| POST   | /api/monitors             | Create monitor         |
| PATCH  | /api/monitors/{id}        | Update monitor         |
| DELETE | /api/monitors/{id}        | Delete monitor         |
| GET    | /api/monitors/{id}/uptime | Uptime samples         |
| GET    | /api/state                | Current state          |
| GET    | /api/events               | Event log              |
| GET    | /api/config               | Get config             |
| PUT    | /api/config               | Update config          |
| POST   | /api/notify/test          | Send test notification |
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
cmd/beacon/          Entry point
internal/
  checks/            HTTP and TCP probes
  config/            Configuration handling
  monitor/           Core monitor logic
  notify/            Notifications
  scheduler/         Job scheduling
  store/             Persistence layer
  sync/              Multi-instance sync
  web/               HTTP server and API
templates/           Web UI templates
static/              CSS and JS
```

## License

MIT License

Copyright (c) 2026 Innokentiy Sokolov
