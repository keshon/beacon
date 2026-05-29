# Backlog after feature expansion

The feature expansion plan (HTTP auth/keyword, email/webhook channels, tri-state notify overrides, Bootstrap JS removal) is implemented.

## Optional follow-ups

| Item | Suggested action |
|------|------------------|
| Store/scheduler/web integration tests | Table-driven tests for store locking, scheduler drops, auth 401/CSRF |
| Static file layout | Move `notify-*.js` under `static/beacon/` for path parity with `window.Beacon` |
| Dashboard badges | Show per-monitor notify override mode (custom/off) on the dashboard |
| Interval warnings in UI | Surface `IntervalWarnings` when creating/editing monitors with short intervals |
| HTTP Basic Auth integration test | Live round-trip test would require a public bind address or injectable transport |

## UI build

After editing `uikit/scss/`, rebuild CSS:

```bash
./tooling/scripts/uikit-build.sh   # Unix
tooling\scripts\uikit-build.bat    # Windows
```
