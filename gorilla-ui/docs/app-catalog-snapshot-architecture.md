# App Catalog snapshot architecture

This document describes the final App Catalog snapshot architecture implemented for issue #254. The goal is to keep catalog reads fast and bounded even when Gorilla managed execution or catalog projection is slow.

## Final data flow

```text
Managed execution
    serialized and potentially long
          |
          v
App Catalog projection
    serialized with managed execution
          |
          v
service-owned last-known-good snapshot
    memory + ProgramData persistence
          |
          v
fast named-pipe snapshot reads
          |
          v
UI
    LocalAppData fallback cache
```

These layers have deliberately different responsibilities:

- managed execution may remain slow;
- snapshot generation may remain slow;
- snapshot serving must remain fast;
- the UI fallback cache is separate from the service snapshot;
- both snapshots and caches are presentation state only;
- `InstallItem` and `RemoveItem` admission still revalidate against current service truth.

Catalog snapshot reads bypass the serialized managed-execution queue. A slow or blocked managed run may make the catalog stale, but it must not make an already-available snapshot read wait behind `execMutex`.

## `ListOptionalInstalls` refresh semantics

Supported App Catalog clients send an explicit `refresh` value.

### `{ "refresh": true }`

The service:

1. returns the current service snapshot immediately;
2. requests or coalesces background snapshot regeneration;
3. does not wait for regeneration before replying.

Startup and manual Refresh use `refresh:true`.

### `{ "refresh": false }`

The service returns the current service snapshot plus refresh state and does not request regeneration.

Polling while a refresh is queued/running and post-operation reconciliation use `refresh:false`.

The refresh coordinator coalesces concurrent requests. Snapshot generation waits for the serialized execution boundary, while the pipe read itself remains independent of that boundary.

## Response semantics

`ListOptionalInstalls` returns `items` plus snapshot/refresh metadata:

| Field | Meaning |
| --- | --- |
| `snapshotAvailable` | Whether the service currently has a usable last-known-good snapshot. |
| `snapshotGeneratedAtUtc` | When the currently returned snapshot was generated. This is the authoritative snapshot freshness timestamp. |
| `refreshState` | Current refresh coordinator state (`Idle`, `Queued`, `Running`, or `Failed`). |
| `refreshRequestedAtUtc` | When the current/most recent refresh request was accepted. |
| `refreshCompletedAtUtc` | When the current/most recent refresh attempt completed. |
| `refreshErrorCode` | Safe, stable failure classification when refresh state is `Failed`; raw refresh errors are not sent to the UI. |

The empty-array cases are intentionally distinct:

```text
snapshotAvailable=false, items=[]
```

means no usable service snapshot exists.

```text
snapshotAvailable=true, items=[]
```

means the authoritative optional catalog is empty.

`snapshotGeneratedAtUtc` is the source freshness timestamp. The time a response happens to arrive at the UI is not a substitute for it.

## Two persistence layers, two failure domains

### Service snapshot

Path:

```text
%ProgramData%\gorilla\app-catalog-snapshot.json
```

The service keeps the current snapshot in memory and persists the last-known-good snapshot under service-owned ProgramData storage. This layer exists so a fast service-owned snapshot remains available when:

- managed execution is busy;
- startup convergence is still running;
- the service has restarted and restored the persisted snapshot.

Persistence is atomic and includes snapshot schema/source validation. Failure to persist a newly generated snapshot does not discard an otherwise valid in-memory snapshot.

### UI cache

The packaged UI also persists its last successfully reconciled presentation state under per-user LocalAppData / packaged-app storage.

This separate layer exists for a different failure domain:

- the service is unavailable;
- the named-pipe connection fails;
- the user should still be able to see the last known UI data.

The service snapshot and UI cache are never mutation authority. Cached action decisions may be stale; the service revalidates every Install/Remove request against current service truth before accepting it.

## Last-known-good publication

Snapshot publication is replace-on-success:

```text
current snapshot A
      |
build candidate B
   /       \
failure   success
  |          |
keep A     publish B
```

A candidate is built completely before publication. If projection fails, the existing snapshot remains active.

Snapshots are published after:

- a successful full managed run;
- targeted Install/Remove when post-operation projection succeeds;
- an explicit background refresh.

Full and targeted managed execution return prepared manifest/catalog state so service-side projection can reuse the state already loaded for execution rather than performing an unrelated second repository retrieval.

Targeted Remove is a special case: the one-time removal manifest is transient execution state. Post-remove projection deliberately uses the persistent service configuration rather than reprojecting from that transient one-time-removal manifest. This keeps the published catalog aligned with future steady-state policy instead of an operation-scoped artifact.

## UI behavior

### Startup

1. Load the usable per-user UI cache, if present.
2. Request `ListOptionalInstalls` with `refresh:true`.
3. Show an available service snapshot immediately, subject to source-freshness precedence against already-presented cached data.
4. If refresh is `Queued` or `Running`, keep existing data visible and present `Refreshing…`.
5. Poll with `refresh:false` while regeneration is in progress.
6. Apply the newer snapshot when it is published and successfully reconciled.

An older restored service snapshot does not replace newer trusted UI-cached presentation merely because the service is reachable.

### Manual Refresh

1. Keep the current catalog visible.
2. Request `refresh:true`.
3. Present `Refreshing…` while refresh is queued/running.
4. Poll with `refresh:false` until the refresh becomes `Idle` or `Failed`.
5. Apply a newly published snapshot when available.

Manual Refresh does not hold a named-pipe request open for the duration of snapshot generation.

### Post-operation reconciliation

Targeted Install/Remove performs service-side postcondition verification and, when the post-operation catalog projection succeeds, publishes that observed snapshot before returning the terminal operation result. The UI then reads with `refresh:false`; it does not immediately trigger an additional full refresh.

## Authority boundary

The snapshot architecture changes how catalog presentation is served, not who owns mutation truth.

- service snapshots are presentation state;
- UI caches are presentation state;
- `actions.install` / `actions.remove` in either layer may become stale;
- every mutation is checked against current service state and policy at admission time;
- post-operation result classification and verification remain service-owned.

This separation is what allows reads to stay responsive without weakening Install/Remove correctness.

## Implementation status

Issue #254 was implemented in staged PRs:

- PR A — complete: neutral managed-execution result boundary;
- PR B — complete: service snapshot foundation and publication;
- PR C — complete: fast snapshot reads and refresh coordination;
- PR D — complete: UI adoption of service snapshot refreshes;
- PR E — documentation and wrap-up of the final architecture.

The original real-world acceptance scenario has also been manually validated: restarting the Gorilla service and opening App Catalog during startup convergence remains responsive, the persisted snapshot appears immediately, and refresh completes later.

Further work such as process/script timeouts, parallel catalog observation, package-global state cleanup, protocol v2, broader UI automation, synthetic long-run harnesses, or benchmarking is intentionally outside this architecture and issue #254.
