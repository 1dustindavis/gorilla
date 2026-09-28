# App Catalog state and action contract

This document describes the **current** optional-software state/action/result contract implemented by Gorilla. The staged history and design decisions that produced it are recorded in [issue #208](https://github.com/1dustindavis/gorilla/issues/208).

The canonical Go types live in `pkg/appcatalog`; the C# client carries matching live-v1 payloads. Shared contract examples/tests exist to keep both sides aligned.

## Protocol status

Production uses the existing newline-delimited named-pipe envelope with `version: "v1"`. The App Catalog work did not require a protocol-version bump.

Current operations are:

- `ListOptionalInstalls`
- `InstallItem`
- `RemoveItem`
- `ListOperations`
- `StreamOperationStatus`

Earlier `v2` examples/design notes in this directory are historical/non-active design material. They must not be read as a pending migration requirement.

## Three independent kinds of state

App Catalog deliberately separates:

| Kind | Meaning | Examples |
| --- | --- | --- |
| Observation | What shared Gorilla detection established on the device | `Absent`, `Installed`, `UpdateAvailable`, `Unknown`, `DetectionFailed` |
| Policy/selection | What administrators require plus the user's optional selection | required install/removal/dependency, selection `None` or `Install` |
| Operation | What one accepted request is doing or how it ended | queued/running/completed plus structured result |

A selected app can be absent. An installed app can be unselected. A failed operation can leave an app present. Connection/cache state is separate again and must not be converted into an installation result.

## Observation authority

The UI receives observation from the Go service. Gorilla uses the shared `pkg/status` implementation used by normal management paths; C# does not implement separate registry/file/script/AppX detection or compare display-version strings to derive state.

`targetVersion` is catalog display metadata. `installedVersion` is observed evidence and may be unavailable. The selected detection check remains authoritative for whether installation work is satisfied and for post-operation verification.

Important consequences:

- `Installed` establishes presence, not necessarily a known version or target satisfaction.
- `UpdateAvailable` requires reliable presence plus applicable unmet version evidence from the selected detection semantics.
- `Unknown` and `DetectionFailed` are not aliases for absence.
- Script checks can establish `Satisfied`/`NotSatisfied` installation requirement while physical presence remains `Unknown`.
- Missing or ambiguous evidence must not fall back to comparing `targetVersion` in the UI.

## Optional selection and policy

The user's local optional selection has two states:

- `None`
- `Install` (`KeepInstalled` in the Go model)

Install persists `Install` selection and requests convergence to the current selected item. Selected optional software therefore participates in normal future Gorilla management/update convergence.

Remove clears the user's local Install selection and performs the required one-time uninstall/withdrawal behavior. It does **not** create persistent user-controlled `managed_uninstalls` policy. A later external reinstall is not repeatedly removed because of the historical App Catalog Remove.

Administrator-authored required install/uninstall policy remains independent and takes precedence. Required dependencies can also block removal.

## Service-owned action decisions

The service owns action eligibility. `ListOptionalInstalls` returns `actions.install` and `actions.remove` decisions containing `allowed` plus a stable reason when denied. Core presents these decisions; it does not reproduce the policy engine.

Cached decisions are not admission authority. Every mutation is revalidated against current service truth before execution.

Common blockers include:

- item not optional;
- administrator policy conflict or required install/uninstall;
- active work for the item;
- invalid local selection;
- unknown/detection-failed state;
- missing install/remove capability;
- required dependency;
- already-selected/already-absent cases where the requested action has no valid meaning.

The underlying service action remains Install or Remove. Core may present contextual user-facing labels such as **Update** for an install/convergence action when observation is `UpdateAvailable`, or **Enable Updates** when adopting an already-installed unselected app.

## Mutation identity and acknowledgement uncertainty

Each Install/Remove request carries a caller-generated `mutationId` that identifies one logical user action.

- repeating the same mutation ID for the same item/action resolves to the original service operation;
- the same mutation ID cannot be reused for a different item/action;
- conflicting concurrent work for the same item is rejected;
- execution remains serialized.

If acknowledgement is uncertain, the client may retry transport using the **same** mutation ID. It must not create a fresh mutation merely because the acknowledgement or connection was lost.

See [app-catalog-recovery.md](app-catalog-recovery.md) for the full recovery rules.

## Operation lifecycle and result

The Go contract expresses generic operation phase as `Queued`, `Running`, `Completed`; the live C# status stream exposes more detailed non-terminal lifecycle states (`Queued`, `Validating`, `Downloading`, `Installing`/`Removing`) and the sole terminal lifecycle state `Completed`.

Success/failure is **not** encoded as terminal lifecycle states. A terminal `Completed` event carries a structured `result` with one of:

- `Succeeded`
- `AlreadySatisfied`
- `Failed`
- `Unverified`
- `Interrupted`

The result also includes a stable code and may include diagnostic detail/message on the wire/presentation path.

Every operation carries stable `operationId`, `itemName`, and `action` identity. `progressPercent` is nullable and remains absent when Gorilla has no measured progress; old fixed milestone percentages are not current semantics.

A successful overall managed execution is not by itself proof that the requested app succeeded. Per-item/dependency execution evidence and postcondition verification determine the structured result.

## Retained operations and Activity

The service owns a bounded in-memory registry of current/recent App Catalog operations and exposes it through `ListOperations`.

Core uses those snapshots to rebuild Activity and active item presentation after navigation, UI relaunch, or status-stream recovery. Activity history preserves the historical operation's own action/result identity; contextual current labels such as Update/Enable Updates are not retroactively substituted into historical operation truth.

The registry is not persisted across a service restart. A service restart therefore does not manufacture a historical result or replay an old mutation. Persistent desired state may later converge through Gorilla's normal managed run.

## Retry

Retry is deliberately **not replay**.

For a retained `Failed`, `Unverified`, or `Interrupted` operation, Core checks the historical item/action against current canonical catalog state, current service-derived action decision, active work, and current retry/admission state.

If eligible, Retry dispatches through the normal current Install/Remove path. That new intent receives a fresh mutation identity and, if accepted, a distinct new `operationId`. The old operation remains unchanged in Activity.

If the service rejects the current action, that rejection is current-action feedback rather than a fabricated terminal operation. A successful manual Refresh recomputes eligibility from fresh service truth.

## Catalog freshness and cache semantics

The UI may render cached `ListOptionalInstalls` data before live refresh for fast/offline startup. Cache state is presentation state, never mutation authority.

Current behavior distinguishes:

- initial loading;
- live/fresh data and last-updated time;
- cached/stale/degraded data after live failure;
- authoritative empty optional assignment;
- no search results;
- no usable cached data;
- live load failure;
- cache-write degradation after otherwise successful live refresh.

A cache write failure must not discard fresh service data or masquerade as a service failure.

## Historical rationale retained from Stage 1

The implementation still follows the key Stage 1 design principles:

- do not infer physical installed state by negating a legacy “action needed” boolean;
- keep observation, policy/selection, and operation separate;
- reuse shared Go detection rather than create UI-specific detectors;
- preserve exact catalog item identity rather than authorize by display name;
- make administrator policy authoritative over user selection;
- treat one-time Remove differently from persistent managed-uninstall policy;
- keep installer execution serialized;
- report uncertainty as uncertainty instead of inventing success/failure;
- keep diagnostic details available while presenting plain-language failures.

The earlier staged language (“Stage 2 must…”, “Stage 3 will…”, “on activation…”) described implementation work that is now complete and is intentionally not normative here.

## Validation authority

Contract behavior is primarily covered in Go and portable Client/Core tests. Windows service/pipe integration validates the real boundary; FlaUI tests cover representative rendered-UI behavior; produced-MSIX validation covers the installed package/service boundary and relaunch/no-replay behavior.

For the current validation commands and Stage 7 scope, see [`../README.md`](../README.md) and [`../ARCHITECTURE.md`](../ARCHITECTURE.md).
