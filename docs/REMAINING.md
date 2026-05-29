# Backlog after optimization pass

The architecture, auth, operations, and frontend cleanup items from the naming/structure review are implemented. Remaining optional work:

## Tests & docs

| Item | Suggested action |
|------|------------------|
| Store/scheduler/web integration tests | Table-driven tests for store locking, scheduler drops, auth 401/CSRF |
| README depth | Expand examples for `GET /api/check-records` and `internal/service` |

## Optional frontend

| Item | Suggested action |
|------|------------------|
| Static file layout | Move `notify-*.js` under `static/beacon/` if you want path parity with the `window.Beacon` namespace |
