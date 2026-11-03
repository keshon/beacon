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

## Startup and state

`cmd/beacon` opens the working directory's `data/` datastore, acquires the server
lock and loads saved configuration before trying the bootstrap JSON file.
A first install can fall back to defaults. Live configuration is held through
`config.Live`; configuration writes are persisted to the store.

A check outcome and the monitor state it produces are persisted through the
storage layer. The current engine is `keshon/datastore`, with a write-ahead log
and snapshots, not the earlier collection of JSON persistence files. Legacy
unconverted files are refused rather than silently ignored.

## Check and notification flow

The scheduler obtains local monitors, or a cluster-backed MonitorSource when
peer mode is enabled. Check implementations run HTTP/TCP probes; status
transitions feed the notification path wired in `main`. Alert delivery has a
bounded queue and local deduplication. It does not provide distributed exactly-once
notification delivery.

HTTP routes live in `internal/server/server.go`. Session/Basic authentication
and CSRF middleware protect application routes; health and token-authenticated
sync have specific exceptions. See [API](docs/api.md).

## Peer behavior



Multi-instance mode is **visibility + opportunistic failover**, not full HA:

- Pull-only peer cache in the `peers` collection
- Dead-peer adoption via sorted ring (one adopter per dead node)
- Export includes adopted monitors for downstream peers
- State merge by latest `LastCheck`

Each node must have synced a peer at least once before it can adopt that peer's monitors.

## Limits

- Quorum / leader election
- CRDT or gossip replication
- Shared external database (Postgres, etcd)
- Automatic monitor replication to every node
- Distributed alert deduplication across nodes
- Build-tag-only single-node binary (runtime gate is enough for now)



Operational configuration and partition limitations are in [Sync](docs/sync.md).
Contributor ownership rules are in [Conventions](docs/conventions.md).
