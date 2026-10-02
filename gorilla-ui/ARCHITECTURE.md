# Gorilla UI Architecture

This document describes the current App Catalog architecture. Historical implementation sequencing lives in [issue #208](https://github.com/1dustindavis/gorilla/issues/208); the current product contract is [docs/app-catalog-contract.md](docs/app-catalog-contract.md), the final snapshot architecture is [docs/app-catalog-snapshot-architecture.md](docs/app-catalog-snapshot-architecture.md), and recovery details are in [docs/app-catalog-recovery.md](docs/app-catalog-recovery.md).

## Runtime layers

Runtime dependency direction is `Gorilla.UI.App -> Gorilla.UI.Core -> Gorilla.UI.Client`. App may also construct the concrete Client implementation at the composition boundary.

- `gorilla-ui/src/Gorilla.UI.App/` owns WinUI 3 XAML, Windows lifecycle, navigation/adapters, cache-path selection, accessibility/UIA wiring, and runtime composition.
- `gorilla-ui/src/Gorilla.UI.Core/` owns platform-neutral presentation and workflow behavior: catalog/detail/activity presentation models, startup/cache coordination, action orchestration, operation tracking/recovery, retry eligibility, and user-facing infrastructure/degraded-state presentation.
- `gorilla-ui/src/Gorilla.UI.Client/` owns the named-pipe client boundary: v1 contracts, transport, serialization/validation, mutation acknowledgement, operation lookup/status streaming, and client diagnostics.
- `gorilla-ui/tests/Gorilla.UI.Core.Tests/` and `gorilla-ui/tests/Gorilla.UI.Client.Tests/` contain the portable behavioral and protocol coverage. Windows/FlaUI tests cover the thin real-UI boundary.

Core targets ordinary `net8.0` and does not depend on WinUI. Client does not depend on Core or App. Detection, action authorization, policy mutation, and installer execution are not UI responsibilities.

## Privilege and execution boundary

App Catalog normally runs as a standard, non-elevated packaged UI and talks over the `gorilla-service` named pipe to the installed Gorilla Windows service, which runs as `LocalSystem`.

The service is authoritative for optional-software observation, policy/action admission, mutation execution, operation identity, retained operation lookup, and the service-owned App Catalog snapshot. Core renders and orchestrates service-owned truth; it must not reimplement detection or authorize an action from cached UI state.

Accepted App Catalog mutations use a targeted managed-execution path behind the existing service boundary. `InstallItem` executes the requested item plus its required install dependency closure; `RemoveItem` executes only the requested uninstall. These operations do not process unrelated managed installs, uninstalls, or updates. Normal CLI execution, service startup convergence, scheduled/periodic convergence, and explicit service Run operations continue to use the full managed convergence path. Both execution modes share Gorilla's manifest/catalog loading, installer/process primitives, status evidence, logging/reporting, cache handling, and service serialization.

The produced-MSIX validation installs the package, verifies the service is registered/running as `LocalSystem`, exercises packaged UI/service communication, and validates relaunch recovery. GitHub-hosted Windows runners use an elevated interactive session, so an explicit medium-integrity/non-elevated UI-process assertion remains a manual validation in a normal desktop session rather than a hosted-CI claim.

## Named-pipe protocol

The active protocol uses newline-delimited UTF-8 JSON envelopes with `version: "v1"`. Current operations are:

- `ListOptionalInstalls`
- `InstallItem`
- `RemoveItem`
- `ListOperations`
- `StreamOperationStatus`

A request/response/event/error envelope carries `requestId` for request correlation and, where applicable, `operationId` for one accepted service operation. Install and Remove requests also carry a caller-generated `mutationId`.

Supported App Catalog clients send an explicit `refresh` value with `ListOptionalInstalls`: `refresh:true` returns the current snapshot immediately and requests/coalesces background regeneration; `refresh:false` returns current snapshot and refresh state without triggering regeneration. Startup and manual Refresh use `true`; polling and post-operation reconciliation use `false`.

`mutationId` identifies one logical user mutation across acknowledgement uncertainty. Repeating the same mutation ID for the same item/action resolves to the original operation; it is not a request to run the installer again. Reusing it for a different item/action is rejected.

See `Gorilla.UI.Client/ProtocolConstants.cs`, `ProtocolPayloads.cs`, the Go service implementation, and [docs/app-catalog-snapshot-architecture.md](docs/app-catalog-snapshot-architecture.md) for the executable contract.

## Optional-software model

App Catalog keeps three concepts separate:

1. **Observation** — what current detection established, such as absent, installed, update available, unknown, or detection failed.
2. **Policy/selection** — administrator requirements plus the user's persistent optional-install selection.
3. **Operation** — one accepted Install or Remove request and its lifecycle/result.

The Go service uses the shared Gorilla catalog/status implementation to build observations and action decisions. C# does not independently inspect registry/files/packages or compare displayed version strings to infer state.

Install persists the local managed-install selection and requests convergence to the selected item. Selected optional software remains subject to normal future Gorilla management/update convergence.

Remove clears the local managed-install selection and performs a one-time uninstall when required and permitted. It does not create persistent user-managed uninstall policy. If software is later reinstalled outside Gorilla, the historical user Remove is not repeatedly enforced. Administrator-controlled managed-uninstall policy remains separate.

The service owns allowed actions and reason codes. Cached actions are presentation data, not admission authority; every new request is revalidated by the service against current truth.

## Operation lifecycle and results

The active lifecycle states are:

- `Queued`
- `Validating`
- `Downloading`
- `Installing` or `Removing`
- `Completed`

`Completed` is the only terminal lifecycle state. Success/failure meaning is carried by the structured terminal `result`, including outcomes such as `Succeeded`, `AlreadySatisfied`, `Failed`, `Unverified`, and `Interrupted` with stable result codes/details. The old model in which `Succeeded`, `Failed`, or `Canceled` were lifecycle states is obsolete.

Every operation carries immutable `operationId`, `itemName`, and action identity. `progressPercent` is nullable and remains indeterminate when Gorilla has no measured progress.

## Tracking, relaunch, and recovery

The service keeps a bounded in-memory operation registry and exposes it through `ListOperations`. Active work is service-owned and independent of App Catalog pages, cards, or process lifetime while the service itself remains running.

Core reconstructs Activity and active item state from retained operation snapshots. On transient status-stream loss it attempts bounded reconnection/reconciliation instead of treating a broken pipe as an installer failure.

Closing and relaunching App Catalog does **not** resubmit unfinished mutations. The relaunched UI asks the service for retained operations and resumes presentation/tracking of the same `operationId`. Installed-product validation asserts relaunch continuity and that no additional `InstallItem` request is submitted.

The registry is not persisted across a service restart. After restart, App Catalog refreshes observed state and does not invent a historical terminal result or blindly replay an installer. Persistent Install selection may naturally converge during a later normal managed run; that is desired-state convergence, not mutation replay. One-time Remove is not automatically repeated.

See [docs/app-catalog-recovery.md](docs/app-catalog-recovery.md) for retention and reconnect details.

## Retry semantics

Retry is a **new user action**, not replay of a historical request.

For a retained unsuccessful operation, Core resolves the current canonical item and current service-derived action decision. If Retry remains eligible, it dispatches through the ordinary current Install/Remove path, which receives a fresh mutation identity and is re-authorized by the service. The historical operation and its `operationId` remain unchanged in Activity.

A service admission rejection is current-action feedback, not a synthetic new operation result. A successful Refresh can make Retry eligible or ineligible as current catalog truth changes.

## Catalog snapshot, cache, freshness, and degraded state

Catalog presentation has two persistence layers with separate failure domains:

- the **service-owned last-known-good snapshot** lives in memory and at `%ProgramData%\gorilla\app-catalog-snapshot.json`; it keeps service reads fast while managed execution/startup convergence is busy and survives service restart;
- the **per-user UI fallback cache** lives under LocalAppData / packaged-app storage and keeps the last known presentation available when the service or named pipe is unavailable.

Neither layer is authoritative for Install/Remove admission.

Snapshot generation remains serialized with managed execution and may be slow. Snapshot serving is a cheap fast path that does not wait behind `execMutex`. The service builds a complete candidate and publishes it only on success; generation failure retains the previous snapshot.

`snapshotGeneratedAtUtc` is the freshness timestamp for service snapshot data. Response receipt time is not freshness. `snapshotAvailable=false` with `items=[]` means no usable service snapshot exists; `snapshotAvailable=true` with `items=[]` means the authoritative catalog is empty.

Startup is cache-first: Core loads usable UI cache, requests `refresh:true`, immediately consumes an available service snapshot when appropriate, then polls `refresh:false` while regeneration is queued/running. Manual Refresh keeps current data visible, requests `refresh:true`, and polls until `Idle` or `Failed`. Post-operation reconciliation reads `refresh:false` because targeted execution has already performed postcondition verification and, when post-operation projection succeeds, published the resulting service snapshot.

An older persisted service snapshot must not replace a newer trusted UI-cached snapshot merely because the service is online. This affects presentation only; mutation admission always uses current service truth.

See [docs/app-catalog-snapshot-architecture.md](docs/app-catalog-snapshot-architecture.md) for full publication, refresh-state, persistence, and UI semantics.

Infrastructure/connection uncertainty is presented separately from app-specific operation failure.

## UI surfaces

The current product has three main surfaces:

- **Catalog** — searchable store-style card grid driven by canonical Core presentation, with contextual primary/secondary actions.
- **Details** — description, available/installed version where known, observation, current action explanation, active operation, latest result, Retry when currently eligible, and optional technical details.
- **Activity** — active and retained recent operations reconstructed from the service registry, including result/recovery presentation and operation-ID-rooted troubleshooting controls.

Stable accessibility/UI Automation identifiers and keyboard/focus behavior are documented in [docs/accessibility-automation.md](docs/accessibility-automation.md).

## Validation layers

The validation model intentionally keeps most permutations below the rendered UI:

- `make verify` — portable Go plus Client/Core validation, including state/action/result policy, protocol validation, presentation, cache/degraded behavior, Retry, and recovery logic.
- `make verify-windows` — Windows service/installer/named-pipe integration.
- `make verify-e2e` — source-built WinUI + FlaUI critical workflows, including keyboard/accessibility/automation boundaries and focused presentation checks.
- `make verify-release GORILLA_RELEASE_EXE=<path> GORILLA_RELEASE_MSIX=<path>` — produced standalone binary plus installed MSIX, real `LocalSystem` service, packaged UI/service communication, representative fixture behavior, relaunch/no-replay recovery, and uninstall cleanup.

Stage 7 presentation validation is deliberately focused rather than exhaustive: representative Catalog and Details/failure surfaces were checked at baseline/light, 150% scaling, dark app mode, and one high-contrast configuration. It is not a broad DPI/theme/resize matrix.

Successful and failure-oriented Windows UI paths retain appropriate screenshots, automation trees, TRX results, logs, and process/service metadata for review/diagnosis.

## Diagnostics

UI client diagnostics are opt-in (`GORILLA_UI_DEBUG=1` or `GORILLA_DEBUG=1`). Service named-pipe trace diagnostics are debug-only, while normal Gorilla process logging remains available through `gorilla.log`. Diagnostic failures must not fail App Catalog operations.

Current log locations and retention behavior are documented in [README.md](README.md).
