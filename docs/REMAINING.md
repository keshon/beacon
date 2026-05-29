# Naming & structure refactor — remaining items

This document lists improvements from the [naming/structure review](https://github.com/keshon/beacon) that were **not** implemented in the structural refactor pass. Use it as a backlog for follow-up work.

## Completed in this pass

- Split `internal/commands/` into `register.go`, `httputil.go`, `monitors.go`, `state.go`, `config_cmd.go`
- Consolidated notification receiver types via `config.TelegramTarget` / `config.DiscordReceiver` and type aliases in `monitor`
- Exported `config.SanitizeReceiverPolicy`, `SanitizeTelegramTargets`, `SanitizeDiscordReceivers`, `ParseDiscordReceiversJSON`
- `monitor.SanitizeNotifyOverride` delegates to config sanitizers
- Renamed `store.Event` → `store.CheckRecord` (`AppendCheckRecord`, `GetCheckRecords`)
- Unified web handlers: `page*` for HTML, `api*` for JSON; grouped into `page_*.go`, `api_*.go`, `api_network.go`, `api_stream.go`
- `window.Beacon` namespace (`notify`, `policy`, `policyModal`, `settings`) with legacy global aliases
- Templates organized under `templates/dashboard/`, `templates/monitors/`, `templates/settings/`, `templates/partials/`

## Not done — recommended next

### Architecture / navigation

| Item | Why it matters | Suggested action |
|------|----------------|------------------|
| HTTP → commandkit indirection | API behavior still lives behind string keys (`monitor:list`) | Colocate REST handlers with store calls, or rename handlers to match command IDs (`apiMonitorList` ↔ `monitor:list`) |
| Rename `monitor.Engine` | Name hides “status evaluation” role | Rename to `StatusEvaluator` or `CheckStateMachine` |
| Rename `sync.Client` | Too generic | `PeerSyncClient` |
| Rename `realtime.Hub` | Package suggests all realtime concerns | `CheckStreamHub` in package `sse` or `stream` |
| Move `checks/hostpolicy.go` | Security rule lives in execution package | `internal/netpolicy` shared by `validate` and `checks` |

### Auth & config

| Item | Suggested action |
|------|------------------|
| `RememberPlainPassword` / `plainAuthPassword` on `Config` | Extract `AuthCredentials` with `SetPassword` / `PasswordForBasicAuth()` |
| CSRF on cookie-authenticated mutations | Token or SameSite=Strict + double-submit |
| Restrict `POST /api/notify/test` | Only credentials already in config / current row |

### Operations

| Item | Suggested action |
|------|------------------|
| `listen` / `workers` hot-reload | Document in UI (partially via `requires_restart`) or implement graceful reload |
| Root `scripts/` vs `cmd/` | Move to `tooling/scripts/` or document in README |
| Go version in Dockerfile vs `go.mod` | Align versions |

### Tests & docs

| Item | Suggested action |
|------|------------------|
| No store/scheduler/web tests | Table-driven tests for store locking, scheduler drops, auth 401 |
| README project layout | Update to reflect new `templates/` tree and `commands/` files |
| API alias `GET /api/events` | Consider deprecating in favor of `check-records` naming in docs only (JSON shape unchanged) |

### Frontend cleanup (optional)

| Item | Suggested action |
|------|------------------|
| Legacy globals `window.BeaconNotify*` | Remove after one release once all templates use `window.Beacon.*` |
| Rename static files | e.g. `notify-ui.js` → loaded as `Beacon.notify` module file name optional |

## Scores after this pass (estimated)

| Dimension | Before | After |
|-----------|--------|-------|
| Naming quality | 6/10 | 7/10 |
| Structural clarity | 7/10 | 8/10 |
