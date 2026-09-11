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

Retention is intentionally bounded:

- at most 512 tracked operations are retained;
- completed operations expire after 24 hours;
- active operations are retained while the service process remains alive;
- pruning also removes the mutation-to-operation admission entry associated with a removed operation.

This registry is not persisted to disk. A service restart therefore ends historical operation tracking and starts with an empty operation registry.

## Service restart semantics

A service restart must not replay an App Catalog operation merely because it was previously active. After restart, Gorilla resumes its normal managed-state convergence from persistent configuration and fresh detection.

That distinction matters for the two mutation types:

- Install persists the user's managed-install selection before execution. A later scheduled or startup convergence may therefore install the app if detection still says work is needed. That is normal desired-state convergence, not replay of the interrupted operation.
- Remove is one-time behavior. It clears the local install selection and does not persist user-requested managed-uninstall policy, so an interrupted historical Remove is not automatically repeated after restart.

Observed state after restart cannot prove whether the historical operation succeeded or failed. The UI must treat lost tracking as uncertainty, refresh current observed state, and avoid inventing a terminal result.

## Connection recovery

The client tracks operation state independently of catalog card instances. A status stream gets one reconnect attempt per recovery cycle. If both attempts fail, the UI reconciles with `ListOperations`:

- terminal operation found: project the terminal result and refresh catalog state;
- active operation found: preserve busy state, wait briefly, and begin another bounded recovery cycle;
- operation no longer retained: report tracking loss separately from installation failure and refresh current observed state.

Catalog refreshes may replace item/card objects, but active operation state is projected back onto the replacement item.

These rules deliberately keep connection/status uncertainty separate from installer failure. A broken status channel does not mean installation failed, and UI cancellation only stops the UI wait; it does not cancel service-side installer work.
