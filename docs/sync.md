# Peer synchronization

Beacon supports optional synchronization between multiple instances for **visibility and opportunistic failover** — not full active/active HA.

When enabled, instances poll peers via `GET /api/sync/export` and cache monitor definitions and state locally. Each node still owns its own monitor definitions in its store; sync does not merge config into the local store automatically.

**What sync provides:**

* Dashboard view of peer monitors and status
* Dead-peer adoption: if a peer stops responding to sync polls for longer than `dead_timeout`, one live node in the ring runs that peer's cached monitors
* Re-export of adopted monitors so indirect peers can discover them downstream
* State merge by latest `LastCheck` timestamp when imports conflict

**Important limitations:**

* A node can only adopt monitors it has previously synced from the dead peer
* Network partitions can cause duplicate checks and duplicate alerts until connectivity is restored
* Monitor definitions are not automatically replicated to every node — plan peer topology so each node syncs from every monitor owner, or accept failover only for previously cached peers
* Indirect nodes (e.g. C peers with B only) display status from their direct peer's export; mesh sync from every owner is recommended for visibility-only nodes
* There is no distributed quorum or leader election; each node decides liveness independently from its own sync success

Configuration:

* self URL
* peer list
* **sync token** — same shared secret on every node (required for reliable sync)
* sync interval
* dead timeout settings

Each node uses the sync token for outbound peer requests (`Authorization: Bearer …` or `X-Beacon-Sync-Token`). Web UI login credentials can differ per node. If no sync token is configured, legacy web Basic Auth is used (all nodes must share the same username and password, and outbound auth breaks after restart when only `password_hash` is stored).

Export endpoint:

```
GET /api/sync/export
```

Requires sync to be enabled on the exporting node and a valid sync token (when configured). The export includes local monitors plus monitors this node has adopted from dead peers.

## Verify and recover

Check `/peers` and `/api/network/status` for successful imports and peer errors.
Test a controlled peer outage and recovery before depending on adoption. Inspect
which monitors were cached and which node is checking them; dashboard visibility
alone does not prove failover or alert delivery.

Restore peer connectivity or correct the shared token before changing ownership.
Partitions can produce duplicate checks and alerts. This is not a quorum-based
service and there is no global duplicate-alert suppression. Preserve each
owner's state using the [backup procedure](running.md#state-backups-and-recovery).
