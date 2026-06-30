# Beacon architecture

One loop, eight packages. Peers are optional.

```
cmd/beacon/           composition root (wiring, alerts, shutdown)
internal/scheduler    when checks run (the loop)
internal/checks       how probes execute
internal/monitor      domain types + status transitions
internal/store        JSON persistence
internal/notify       alert delivery
internal/config       settings
internal/web          HTTP UI + API
internal/cluster      optional peer sync + adoption (off by default)
internal/netpolicy    SSRF guard for outbound checks
```

## Principles (read before adding a package or file)

These apply to every change — not just the initial layout.

### Packages are decision-makers, not folders

A new `internal/` package should own **one kind of decision** that nothing
else should make. If the package only forwards calls (`service` calling
`store`, `commands` calling `service`), it is a thin layer — delete it and
call the real owner directly.

Ask before creating a package:

- Who is the **single owner** of this behavior?
- Would deleting this package force a real design choice elsewhere, or just
  inline three lines?

If the answer is "just inline three lines", do not add the package.

### Prefer one domain package over many small files

Within a package, **one mental model beats many tiny files**. A ~800-line
`store.go` is fine when it is one persistence domain. Split a file only when
the split clarifies ownership (e.g. `scheduler/scheduler.go` vs
`scheduler/source.go` — loop vs monitor listing seam).

Do **not** split because "files should be short." Split because two concerns
would confuse a reader if they shared a file.

### No duplicate schedulers, no duplicate loops

There is exactly one monitoring loop (`internal/scheduler`). Optional
behavior attaches through **interfaces** (`MonitorSource`), not a second
goroutine that also decides when checks run. The same rule applies elsewhere:
one alert pipeline in `main`, one persistence path in `store`.

### Optional capabilities plug in at the root

Peer mode is **one optional package** (`cluster`) wired from `main` when
`network.enabled`. Core packages (`scheduler`, `store`, `web`) must not import
`cluster`. If a feature is off by default, isolate it behind an interface and
a single wiring block in `cmd/beacon` — do not spread `if cluster` through
the tree.

### Backend and provider quirks stay local

Code that exists only because Telegram rate-limits, SMTP is finicky, or HTTP
probes need SSRF guards belongs in `notify`, `checks`, or `netpolicy` — not in
`scheduler`, `web`, or `main`. The loop asks "when"; local packages answer
"how" for their backend.

### Composition root stays fat

`cmd/beacon/main.go` owns cross-cutting wiring: alert queue, dedup, email
guard, startup-down notify, flock, shutdown order, cluster attach. That is
intentional. Do not hide wiring in a new `internal/wire` or `internal/app`
package unless `main` has genuinely grown hard to navigate — and even then,
prefer `cmd/beacon/wire.go` beside `main`, not a new internal package.

### Web is transport, not domain

`web` handles HTTP, auth, CSRF, templates, and JSON shapes. It calls `store`
and `monitor` for data and validation. Presentation logic that requires peer
ring math or export assembly lives in `cluster` view helpers, not in page
handlers.

### CLI is not a second server

CLI commands are registered via [`github.com/keshon/command`](https://github.com/keshon/command) in [`internal/command/`](internal/command/) and dispatched from [`cmd/beacon/cli_adapter.go`](cmd/beacon/cli_adapter.go). Commands talk to `store` directly. A `WithDataDirLock` middleware prevents concurrent CLI + server access. Flock is not provided by datastore (in-process mutex only).

### Add packages for needs, not for futures

Do not introduce abstractions for capabilities you do not have yet (shared DB,
quorum, CRDT, build-tag-only binaries). See **Anti-roadmap** below. When a
real need appears, add the smallest package that owns that decision — and
update this document if the package map changes.

### Tests follow code; they do not drive structure

`*_test.go` files and `testdata/` do not justify a new production package.
Keep tests next to the code they exercise.

## Rules

1. **Only scheduler schedules checks.** Nothing else enqueues probes.
2. **cluster is a MonitorSource adapter, not a second scheduler.** When `network.enabled`, main wires `cluster.Runtime` as the monitor source. When off, scheduler uses `LocalSource` only.
3. **web does not implement ring math.** Dashboard and network status call `cluster.Runtime` view helpers.
4. **Backend quirks stay local.** HTTP/TCP details in `checks`; alert channel behavior in `notify`; SSRF in `netpolicy`.
5. **Composition root stays fat.** Alert dedup, email guard, startup-down notify, flock, and goroutine wiring live in `cmd/beacon/main.go`.

## cluster semantics (honest)

Multi-instance mode is **visibility + opportunistic failover**, not full HA:

- Pull-only peer cache in `peer_data.json`
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
