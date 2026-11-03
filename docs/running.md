# Running Beacon

Run from the repository root, or keep `templates/` and `static/` alongside a
built binary and start from that directory. Data is stored under the working
directory's `data/`. Go 1.25+ is required by [go.mod](../go.mod) for source builds.

## First start

```bash
git clone https://github.com/keshon/beacon.git
cd beacon
go run ./cmd/beacon
```

Open `http://localhost:8080`. A fresh store without a valid bootstrap file uses
`admin` / `admin`. The default listener is `:8080`, not loopback-only; keep the
service private until you have changed credentials and configured access.
Defaults are saved to the datastore, not written out as a new `config.json`.

To seed another configuration file into a fresh store:

```bash
go run ./cmd/beacon /path/to/config.json
```

Existing saved configuration takes precedence. A missing or unreadable bootstrap
file can fall back to defaults; verify the active settings after first start.
Do not delete `data/` just to force a configuration file to be reread.

## Verify the installation

Add an HTTP monitor for an endpoint you control, confirm a successful result
and a history entry, then configure a notification receiver and test it. A
receiver Test sends a real message, including a template preview before Save.
Test failure and recovery on a disposable target before relying on alerts.

`GET /api/health` is public and shallow; it does not demonstrate that monitored
targets are reachable or alerts can be delivered. The [API guide](api.md)
describes authentication and automation.

## Configuration

Use Settings or the configuration API for a running installation. A JSON file
only seeds configuration when the datastore has no saved configuration.

| Area     | Notes                                              |
| -------- | -------------------------------------------------- |
| Server   | Listen address, worker pool size, default interval |
| Auth     | HTTP basic auth for UI and API                     |
| Telegram | Up to 5 bot token + chat ID pairs                  |
| Discord  | Up to 5 webhook URLs                               |
| Email    | Global SMTP + up to 5 recipient addresses          |
| Webhook  | Up to 5 generic HTTP POST endpoints                |
| Sync     | Multi-instance monitor/state synchronization       |

Configuration lives in the datastore under `data/`, relative to the working
directory. A selected `config.json` is a bootstrap input, not a live override.
The positional config argument changes that input path, not the data directory.

Example bootstrap `config.json` (receiver values are placeholders):

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
      "down": "DOWN\\n{{message}}\\nTime: {{time}}\\nFrom: {{node}}\\n{{name}}",
      "recovered": "Service RECOVERED\\n{{message}}\\nTime: {{time}}\\nFrom: {{node}}\\n{{name}}"
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

For HTTP monitors, expand **HTTP options** to set Basic Auth credentials (stored server-side; password is never shown in the UI after save) and an optional response **keyword** (must appear in the body, or enable **Must not contain** to invert). Multi-word keywords match when all words appear in order; spacing and HTML between words is ignored (e.g. `hello world` matches `hello <b>world</b>`).

## Web UI

| Route        | Purpose                                          |
| ------------ | ------------------------------------------------ |
| `/dashboard` | Status overview, live updates, and monitor CRUD  |
| `/monitors`  | Redirects to `/dashboard` (legacy bookmark)      |
| `/settings`  | Configuration and sync                           |
| `/login`     | Authentication                                   |

The dashboard also links to monitor details, incidents, summaries, peers and
notification history. Use [peer synchronization](sync.md) only after a
single-node monitor and alert path works.

## Binary and container deployment

```bash
go build -o beacon ./cmd/beacon
./beacon
```

The binary loads templates/static files from disk; keep those directories
available in the working directory. The supplied Dockerfile copies them into
the image. It expects a build context containing the source in `src/`, not the
repository root as the original build example implied.

From `docker/`, with a reviewed copy of the source prepared in `src/`:

```bash
docker build -t beacon-image .
```

[Dockerfile](../docker/Dockerfile) defines the image;
[Compose](../docker/docker-compose.yml) refers to `${ALIAS}-image`, mounts
`./data` at `/usr/project/data`, publishes host port `3017` to container port
`8080`, and expects an external `proxy` network plus an existing Traefik setup.
Set `ALIAS` to match your image and `HOST` to the intended HTTP hostname.
Review these deployment-specific settings before running Compose.

The legacy `docker/build-n-deploy.sh` sources `.env`, replaces `src/` in clone
mode, stops containers, removes the old image and prunes dangling images.
It is a mutating maintenance script, not a required first-start command.

## State, backups and recovery

One server owns a data directory at a time. Back up the entire `data/` directory
with the process stopped; preserve matching source/binary, templates and static
assets separately. Backups can contain target credentials, receiver tokens and
configuration secrets. Keep them private.

Restore into private staging with writers stopped, review active configuration
and start the matching release. Verify monitors, history and alert delivery.
A restore discards newer changes. Older JSON persistence directories are refused
by the current storage guard; no conversion command ships in `cmd/`. Preserve
the old data and arrange a reviewed conversion rather than deleting the files
to make startup succeed.
