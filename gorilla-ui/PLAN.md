# Gorilla UI Plan

## Goal
Create a Gorilla UI app that runs as a standard user, communicates with the Gorilla service via named pipes, and delivers a Store-like experience with Managed Software Center-style functionality.

## Repository/Branch Constraints
- UI development branch: `gorilla-ui`
- UI code location: `gorilla-ui/`

## First Release Scope
- Display available items (Gorilla `option_installs`)
- Allow users to install items
- Allow users to remove items
- Display status for install/remove operations
- On startup, display cached `ListOptionalInstalls` results first, then immediately refresh from service

## Technical Direction
- UI framework: WinUI 3
- Language/runtime: C# on .NET 8 LTS
- Windows stack: Windows App SDK (stable channel)
- IPC: Named pipes between UI client and Gorilla service

## Development Workflow
- Hybrid workflow:
  - macOS: planning, protocol design, docs, CI/workflow authoring, non-UI code where practical
  - Windows VM: WinUI build/run/debug, XAML iteration, packaging validation
- Prefer CLI-first workflow:
  - `git`, `dotnet`, `msbuild`, `pwsh`, `gh`
  - Use GUI IDE (Visual Studio) only when necessary (XAML diagnostics/hot reload)

## Initial API/IPC Plan (v0)
Define a versioned named-pipe contract with correlation IDs for request/response and status tracking.

Planned operations:
- `ListOptionalInstalls`
- `InstallItem`
- `RemoveItem`
- `StreamOperationStatus` (stream of status/progress updates)

## Reliability/Supportability Baseline
- Structured logging in both UI and service with operation IDs
- Reconnect/timeouts/cancellation handling in pipe client
- Stub/fake service mode to enable UI development without full service dependency
- `ListOptionalInstalls` should return a JSON-safe subset DTO, not full internal item objects (some source fields may be non-JSON-compatible)
- `ListOptionalInstalls` should include per-item status fields (`isInstalled`, status enum, status timestamp, optional last operation id)

## Validation Strategy
- Local: command-line build/test flow
- Automated: Windows GitHub Actions lane for build and tests
- Integration: Windows smoke tests for list/install/remove/status pipe flows
- Keep `cmd/gorilla` CLI service-message commands updated in lockstep with UI pipe API changes for testing/debugging
- Backward compatibility for existing CLI service-message behavior is not required during this UI/API iteration

## Completed Milestones
- WinUI app scaffold created under `gorilla-ui/src/Gorilla.UI.App/` and added to `gorilla-ui/Gorilla.UI.sln`.
- App startup is wired to `HomePage` and `HomeViewModel` with named-pipe client/cache composition.
- `HomePage` async handlers now fail safely and surface errors in UI warning banner.
- Template source files under `gorilla-ui/src/Gorilla.UI.App/template/` are excluded from app compilation.
- Windows VM packaging/install workflow is scripted:
  - `gorilla-ui/tools/build-signed-msix.ps1`
  - `gorilla-ui/tools/install-signed-msix.ps1`
- Service named-pipe handling now flushes pipe buffers before disconnect to improve response/event delivery reliability.
- UI client diagnostics are debug-gated and do not create local log directories when diagnostics are disabled.

## TODOs (Priority Order)
1. Remove `os.Exit` usage from service-facing code paths.
   - Scope: convert fatal exits in runtime call paths into typed errors that bubble back to service handlers.
   - Primary targets: `pkg/manifest`, `pkg/catalog`, and any call chains used by `cmd/gorilla` + `pkg/service`.
   - Done when: malformed manifest/catalog/download failures return errors, service stays alive, and tests cover non-crash behavior.

2. Define a long-term diagnostics strategy for UI + service logs.
   - Scope: agree on default-off/on behavior, log file locations, retention policy, and rotation/truncation behavior.
   - Primary targets: `gorilla-ui/README.md`, `gorilla-ui/ARCHITECTURE.md`, and service logging notes/config docs.
   - Done when: docs specify exact behavior and code follows that policy without creating noisy logs by default.

3. Replace placeholder `StreamOperationStatus` terminal behavior with real async lifecycle events, and add Windows CI coverage for named-pipe reliability.
   - Scope: emit realistic state transitions (`Queued` -> ... -> terminal) tied to actual operation execution and correlate via `operationId`.
   - Primary targets: `pkg/service/run_windows.go`, `pkg/service/common.go`, `pkg/service/run_windows_test.go`, Windows workflows.
   - Done when: stream produces non-placeholder progress/events in live runs and Windows CI validates stream reliability.

4. Verify `ListOptionalInstalls` load + cache-first startup behavior with live service data.
   - Scope: validate cached render first, then refresh/replace behavior, including stale-data warning behavior.
   - Primary targets: `gorilla-ui/src/Gorilla.UI.App/ViewModels/HomeViewModel.cs`, cache store/coordinator classes, manual test docs.
   - Done when: startup flow is confirmed on Windows VM with reproducible test notes.

5. Validate `InstallItem` against real service responses.
   - Scope: ensure operation acceptance, `operationId` handling, and item state updates map to actual service behavior.
   - Primary targets: `pkg/service`, `gorilla-ui/src/Gorilla.UI.Client`, `gorilla-ui/src/Gorilla.UI.App`.
   - Done when: live install flow works end-to-end and expected output is documented/tested.

6. Validate `RemoveItem` against real service responses.
   - Scope: same as install validation, including operation acceptance and state convergence to removed/not installed.
   - Primary targets: `pkg/service`, `gorilla-ui/src/Gorilla.UI.Client`, `gorilla-ui/src/Gorilla.UI.App`.
   - Done when: live remove flow works end-to-end and expected output is documented/tested.

7. Validate `StreamOperationStatus` UI reflection end-to-end.
   - Scope: confirm stream updates are visible in UI state for success/failure/cancel paths.
   - Primary targets: `gorilla-ui/src/Gorilla.UI.App/ViewModels/HomeViewModel.cs`, `Services/OperationTracker.cs`.
   - Done when: each terminal state is represented correctly in the UI and covered by tests/manual checks.

8. Confirm `cmd/gorilla` service-message commands cover current UI protocol operations.
   - Scope: keep CLI command grammar/output aligned with `ListOptionalInstalls|InstallItem|RemoveItem|StreamOperationStatus`.
   - Primary targets: `pkg/service/common.go`, `pkg/service/client_windows.go`, `cmd/gorilla/main.go` + tests.
   - Done when: CLI can drive all current operations and outputs useful debug info for each.

9. Keep CLI behavior aligned as protocol contracts evolve.
   - Scope: enforce lockstep updates whenever envelope shape/payload fields/operation semantics change.
   - Primary targets: UI client contracts, service protocol types, CLI mapping layer, contract docs/examples.
   - Done when: protocol change PRs include corresponding CLI + tests + docs updates.

10. Improve item-level status UX for in-progress and terminal operation states.
   - Scope: surface clear pending/progress/success/failure states per item, not just generic status text.
   - Primary targets: `gorilla-ui/src/Gorilla.UI.App/Models/UiOptionalInstallItem.cs`, view models, `Views/HomePage.xaml`.
   - Done when: per-item state is visually distinct and stable during operation streaming.

11. Distinguish cached-data banner from action failure banner.
   - Scope: separate stale-data/service-warning messaging from user-initiated action errors.
   - Primary targets: `HomeViewModel` + `HomePage` bindings.
   - Done when: startup refresh issues and install/remove failures render in different UI channels/messages.

12. Add a manual refresh button for `ListOptionalInstalls`.
   - Scope: allow retry refresh without app restart and preserve safe cancellation/error handling.
   - Primary targets: `Views/HomePage.xaml`, `Views/HomePage.xaml.cs`, `ViewModels/HomeViewModel.cs`.
   - Done when: refresh can be triggered manually and updates list/banner state correctly.

13. Add Gorilla UI/.NET validation (`make ui-test`) to appropriate GitHub Actions pipelines.
   - Scope: ensure PR/main pipelines run .NET client tests alongside existing Go checks where relevant.
   - Primary targets: `.github/workflows/*.yml`.
   - Done when: CI executes `make ui-test` in targeted workflows and failures block merges/releases as intended.

14. Prepare long-lived optional-install fixtures for UI install/remove smoke tests.
   - Scope: reusable installer/uninstaller payloads and manifests/catalogs for stable VM and CI smoke runs.
   - Primary targets: `integration/windows/`, `utils/manual-test/`, fixture docs.
   - Done when: fixture set supports repeatable install/remove flows without ad hoc setup.

15. Review service execution model for safe concurrency improvements.
   - Scope: evaluate where parallelism helps (queueing, operation tracking, stream delivery) without violating service safety.
   - Primary targets: `pkg/service/run_windows.go` and command execution scheduling paths.
   - Done when: proposal/design notes exist and any adopted concurrency changes are covered by reliability tests.

16. Start Gorilla service automatically after installation.
   - Scope: define install UX and implement post-install start behavior with clear idempotent error handling.
   - Primary targets: `pkg/service/manage_windows.go`, CLI output/messages, install docs.
   - Done when: `serviceinstall` results in a running service (or actionable error) and behavior is documented/tested.
