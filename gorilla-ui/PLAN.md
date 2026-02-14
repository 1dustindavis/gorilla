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

## Immediate Next Steps
1. Implement minimal pipe harness behavior in `gorilla-ui/tools/PipeHarness/` using the shared client/protocol code.
2. Implement cache-first startup behavior for `ListOptionalInstalls`:
   - Add JSON cache file reader/writer for latest list result.
   - Load cache on app startup and render immediately.
   - Trigger immediate live refresh and update both UI and cache.
3. Complete WinUI app scaffolding/wiring:
   - Generate actual WinUI app project on Windows VM.
   - Wire views/viewmodels to the tested client APIs and cache-first startup flow.

## Later TODOs
- Update GitHub Actions workflows to include Gorilla UI/.NET validation (`make ui-test`) in the appropriate pipelines.
