# Beacon architecture

Domain subsystems on top, infrastructure flat below. Peers are optional.

```
cmd/beacon/                    composition root (wiring, alerts, shutdown)
internal/monitor/              domain types + status transitions
internal/monitor/checks        how probes execute
internal/monitor/scheduler     when checks run (the loop)
internal/monitor/runner        monitor mutations (add/update/delete/list)
internal/storage               datastore persistence + data-dir flock
internal/notify                alert delivery
internal/config                settings
internal/server                HTTP wiring (Routes)
internal/server/api            REST JSON handlers
internal/server/page           HTML page handlers
internal/server/middleware     session auth + CSRF
internal/server/stream         live check SSE hub
internal/cluster               optional peer sync + adoption (off by default)
internal/netpolicy             SSRF guard for outbound checks
```

## Principles (read before adding a package or file)

These apply to every change — not just the initial layout.

### Packages are decision-makers, not folders

A new `internal/` package should own **one kind of decision** that nothing
else should make. If the package only forwards calls (`service` calling
`storage`, `runner`), it is a thin layer — delete it and
call the real owner directly.

Ask before creating a package:

- Who is the **single owner** of this behavior?
- Would deleting this package force a real design choice elsewhere, or just
  inline three lines?

If the answer is "just inline three lines", do not add the package.

### Prefer one domain package over many small files

Within a package, **one mental model beats many tiny files**. A ~800-line
`storage/store.go` is fine when it is one persistence domain. Split a file only when
the split clarifies ownership (e.g. `monitor/scheduler/scheduler.go` vs
`monitor/scheduler/source.go` — loop vs monitor listing seam).

Do **not** split because "files should be short." Split because two concerns
would confuse a reader if they shared a file.

### No duplicate schedulers, no duplicate loops

There is exactly one monitoring loop (`internal/monitor/scheduler`). Optional
behavior attaches through **interfaces** (`MonitorSource`), not a second
goroutine that also decides when checks run. The same rule applies elsewhere:
one alert pipeline in `main`, one persistence path in `storage`.

### Optional capabilities plug in at the root

Peer mode is **one optional package** (`cluster`) wired from `main` when
`network.enabled`. Core packages (`monitor/scheduler`, `storage`, `server`) must not import
`cluster`. If a feature is off by default, isolate it behind an interface and
a single wiring block in `cmd/beacon` — do not spread `if cluster` through
the tree.

### Backend and provider quirks stay local

Code that exists only because Telegram rate-limits, SMTP is finicky, or HTTP
probes need SSRF guards belongs in `notify`, `monitor/checks`, or `netpolicy` — not in
`monitor/scheduler`, `server`, or `main`. The loop asks "when"; local packages answer
"how" for their backend.

### Composition root stays fat

`cmd/beacon/main.go` owns cross-cutting wiring: alert queue, dedup, email
guard, startup-down notify, shutdown order, cluster attach. That is
intentional. Do not hide wiring in a new `internal/wire` or `internal/app`
package unless `main` has genuinely grown hard to navigate — and even then,
prefer `cmd/beacon/wire.go` beside `main`, not a new internal package.

### Server is transport, not domain

`server` wires HTTP in [`server.go`](internal/server/server.go); handlers live in
[`server/api/`](internal/server/api/), [`server/page/`](internal/server/page/),
and [`server/middleware/`](internal/server/middleware/). It calls `storage`,
`monitor`, and `monitor/runner` for data and mutations. Presentation logic that requires peer
ring math or export assembly lives in `cluster` view helpers, not in page
handlers.

### monitor/runner is the application port for mutations

`monitor/runner` owns create/update/delete/list flows used by the HTTP API.
It validates via `monitor`, persists via `storage`, and is the single place for
monitor lifecycle changes. Read-only queries (dashboard, settings GET) stay in
`server/page` or `server/api` and call `storage` directly.

### Automation uses the HTTP API

There is no separate CLI binary mode. Scripts and integrations call the REST API
(`curl` with Basic auth, or session + CSRF from the web UI). Data-dir flock in
[`internal/storage/`](internal/storage/) (`AcquireDirLock`) prevents two server
processes from sharing the same `data/` directory.

### Add packages for needs, not for futures

Do not introduce abstractions for capabilities you do not have yet (shared DB,
quorum, CRDT, build-tag-only binaries). See **Anti-roadmap** below. When a
real need appears, add the smallest package that owns that decision — and
update this document if the package map changes.

### Tests follow code; they do not drive structure

`*_test.go` files and `testdata/` do not justify a new production package.
Keep tests next to the code they exercise.

## Rules

1. **Only monitor/scheduler schedules checks.** Nothing else enqueues probes.
2. **cluster is a MonitorSource adapter, not a second scheduler.** When `network.enabled`, main wires `cluster.Runtime` as the monitor source. When off, scheduler uses `LocalSource` only.
3. **server does not implement ring math.** Dashboard and network status call `cluster.Runtime` view helpers.
4. **Backend quirks stay local.** HTTP/TCP details in `monitor/checks`; alert channel behavior in `notify`; SSRF in `netpolicy`.
5. **Composition root stays fat.** Alert dedup, email guard, startup-down notify, and goroutine wiring live in `cmd/beacon/main.go`. Data-dir flock is acquired via `storage.AcquireDirLock`.

## cluster semantics (honest)

Multi-instance mode is **visibility + opportunistic failover**, not full HA:

- Pull-only peer cache in the `peers` collection
- Dead-peer adoption via sorted ring (one adopter per dead node)
- Export includes adopted monitors for downstream peers
- State merge by latest `LastCheck`

Each node must have synced a peer at least once before it can adopt that peer's monitors.

## Anti-roadmap (deliberately not built)

- Quorum / leader election
- CRDT or gossip replication
- Shared external database (Postgres, etcd)
- Automatic monitor replication to every node
- Distributed alert deduplication across nodes
- Build-tag-only single-node binary (runtime gate is enough for now)

Add these only when a concrete user need appears, not speculatively.
