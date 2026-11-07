# App Catalog operation recovery

This document records the Stage 4 recovery decisions for [#208](https://github.com/1dustindavis/gorilla/issues/208).

## Mutation identity and duplicate handling

Install and Remove requests carry a caller-generated `mutationId` that is stable for one logical user action. The service is authoritative for admission:

- the same `mutationId` for the same item/action resolves to the original operation;
- reusing a `mutationId` for a different item/action is rejected;
- a different mutation that conflicts with an active operation for the same item is rejected;
- installer execution remains serialized.

The UI may retry an acknowledgement whose outcome is uncertain, but it must reuse the same `mutationId`. It must not create a new mutation merely because the acknowledgement or connection was lost.

## Operation tracking and retention

Operation tracking is service-process state, independent of page or card instances. The service keeps a bounded in-memory registry of operations and exposes it through `ListOperations` so the UI can recover after navigation, relaunch, or a transient pipe disconnect.

Retention is intentionally bounded for completed history while preserving all active work:

- completed operations are pruned as needed to target a maximum registry size of 512 entries;
- completed operations also expire after 24 hours;
- active operations are never pruned solely to satisfy the 512-entry target and remain retained while the service process is alive, so the registry can temporarily exceed 512 entries if more than 512 operations are simultaneously active;
- pruning also removes the mutation-to-operation admission entry associated with a removed operation.

This registry is not persisted to disk. A service restart therefore ends historical operation tracking and starts with an empty operation registry.

Retained status events keep their original event timestamps. Stream replay uses those timestamps, and each `ListOperations` snapshot includes `timestampUtc` for its latest status. Clients connecting to an older service that omits this field fall back to the response timestamp.

## Service restart semantics

A service restart must not replay an App Catalog operation merely because it was previously active. After restart, Gorilla resumes its normal managed-state convergence from persistent configuration and fresh detection.

That distinction matters for the two mutation types:

- Install persists the user's managed-install selection before execution. A later scheduled or startup convergence may therefore install the app if detection still says work is needed. That is normal desired-state convergence, not replay of the interrupted operation.
- Remove is one-time behavior. It clears the local install selection and does not persist user-requested managed-uninstall policy, so an interrupted historical Remove is not automatically repeated after restart.

Observed state after restart cannot prove whether the historical operation succeeded or failed. The UI must treat lost tracking as uncertainty, refresh current observed state, and avoid inventing a terminal result.

## UI scope: best-effort status, not an audit log

App Catalog is a convenience interface for the **currently observed installation state** and for starting user actions. Live progress and recent Activity are best-effort, transient presentation of service events, not a durable audit record. The UI must never claim success or failure without a terminal result.

When operation tracking becomes unavailable, it is acceptable to lose progress or recent Activity and show an unknown outcome instead of preserving an indefinitely busy card. A refreshed catalog observation answers what is installed *now*, not whether a particular past action completed. Gorilla's normal execution logs and the latest `GorillaReport.json` (in `app_data_path`) support troubleshooting; the report is overwritten by subsequent runs and is not a historical audit log either.

The service, not the UI, remains responsible for action authorization, duplicate admission, and preventing conflicting work. UI cancellation or status loss never cancels the service's operation.

## Connection recovery

The client tracks operation state independently of catalog card instances. A status stream gets one reconnect attempt per recovery cycle. If both attempts fail, the UI reconciles with `ListOperations`:

- terminal operation found: project the terminal result and refresh catalog state;
- active operation found: preserve busy state, wait briefly, and begin another bounded recovery cycle;
- operation no longer retained: report tracking loss separately from installation failure and refresh current observed state.

Catalog refreshes may replace item/card objects, but active operation state is projected back onto the replacement item.

If both status streaming and the reconciliation lookup fail, the UI stops treating that operation as actively tracked: it clears the stale local busy/activity projection, displays an **outcome unknown** warning, and attempts to read current catalog state. This is not a success or failure result. Manual Refresh updates the observed catalog state; it does not reconstruct a historical outcome or automatically reissue the mutation. The service independently rejects conflicting new actions while an operation is actually active.

These rules deliberately keep connection/status uncertainty separate from installer failure. A broken status channel does not mean installation failed, and UI cancellation only stops the UI wait; it does not cancel service-side installer work.
