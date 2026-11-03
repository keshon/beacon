# HTTP API

Monitor/configuration endpoints require HTTP Basic or a session cookie.
`/api/health` is public; sync export has its own token behavior described in
[Sync](sync.md). Cookie-authenticated `POST`/`PUT`/`PATCH`/`DELETE` requests must include the `X-CSRF-Token` header matching the `beacon_csrf` cookie (see `static/js/beacon.js`).

Example with Basic auth:

```bash
curl -u admin http://localhost:8080/api/monitors
curl -u admin -H 'Content-Type: application/json' \
  -d '{"name":"API","type":"http","target":"https://example.com"}' \
  http://localhost:8080/api/monitors
```

**Target format:** HTTP monitors need a full URL (`http://` or `https://`). TCP monitors need `host:port` with no scheme (e.g. `db.local:5432`). The API and UI validate targets on create/update.

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

Additional action endpoints are registered in
[`internal/server/server.go`](../internal/server/server.go):

| Method | Path | Action |
|---|---|---|
| POST | `/api/monitors/{id}/check` | Request an immediate check |
| POST | `/api/monitors/{id}/mute` | Update muting |
| POST | `/api/monitors/{id}/ack` | Acknowledge an incident |
| POST | `/api/peers/sync` | Request peer synchronization |
| GET | `/api/uptime` | Batch uptime query |

Use the handler types under `internal/server/api/` for request/response fields.
This table is navigation, not a separately versioned API schema. Mutation
requests change live state; verify the resulting monitor/configuration and
check history after automation.
