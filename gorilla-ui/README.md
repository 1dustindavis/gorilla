# Gorilla UI

This folder contains all UI app related code for the Gorilla WinUI client.

Tooling requirements:
- macOS development: `dotnet-sdk@8`
- Windows VM development: Visual Studio 2022 with WinUI/Windows App SDK tooling and .NET 8 SDK

Current workspace:
- Solution file: `gorilla-ui/Gorilla.UI.sln`
- Included projects:
  - `gorilla-ui/src/Gorilla.UI.Client/Gorilla.UI.Client.csproj`
  - `gorilla-ui/src/Gorilla.UI.Core/Gorilla.UI.Core.csproj`
  - `gorilla-ui/tests/Gorilla.UI.Client.Tests/Gorilla.UI.Client.Tests.csproj`
  - `gorilla-ui/tests/Gorilla.UI.Core.Tests/Gorilla.UI.Core.Tests.csproj`
  - `gorilla-ui/tools/PipeHarness/PipeHarness.csproj`

Architecture boundary:
- `Gorilla.UI.App` owns XAML, WinUI lifecycle, navigation/adapters, and Windows runtime composition.
- `Gorilla.UI.Core` owns platform-neutral presentation and workflow behavior such as view models, presentation models, startup/cache coordination, and install/remove operation tracking.
- `Gorilla.UI.Client` owns the named-pipe protocol/client boundary: contracts, transport, serialization/validation, and client diagnostics.
- Dependency direction is `Gorilla.UI.App -> Gorilla.UI.Core -> Gorilla.UI.Client`; Core targets ordinary `net8.0` and must not reference `Microsoft.UI.Xaml` or Windows-specific target frameworks.
- Filesystem cache paths are chosen by App; the JSON cache implementation is platform-neutral and lives in Core so its behavior remains covered by portable tests.
- The committed App/Core/Client projects are the source of truth; there is no generated shadow template copy of application code.

Validation commands:
- `make ui-lint` runs `dotnet build -warnaserror` for the portable client/core projects, their test projects, and PipeHarness.
- `make ui-test` runs both portable .NET test projects:
  - `gorilla-ui/tests/Gorilla.UI.Client.Tests/Gorilla.UI.Client.Tests.csproj`
  - `gorilla-ui/tests/Gorilla.UI.Core.Tests/Gorilla.UI.Core.Tests.csproj`
- `make verify` therefore exercises both protocol/client tests and WinUI-independent presentation/workflow tests. The portable commands can be run on macOS/Linux as well as Windows; CI runs them once on Windows.
- Windows UI tests (FlaUI):
  - CI workflow: `.github/workflows/windows-ui-test.yml`
  - Runner: `windows-2025`
  - Behavior: non-blocking (`continue-on-error: true`) with up to 3 attempts
  - Artifacts: TRX results and failure screenshots/logs from `gorilla-ui/tests/Gorilla.UI.App.WindowsUiTests`
  - Local run (Windows):
    - Build app:
      - `dotnet build gorilla-ui/src/Gorilla.UI.App/Gorilla.UI.App.csproj -c Release -p:Platform=x64`
    - Set app path and run Windows UI tests:
      - `$env:GORILLA_UI_APP_EXE = "<path-to-Gorilla.UI.App.exe>"`
      - `dotnet test gorilla-ui/tests/Gorilla.UI.App.WindowsUiTests/Gorilla.UI.App.WindowsUiTests.csproj -c Release`
- Optional local autofix: use `dotnet format` against the relevant portable project.

UI automation and accessibility contract:
- Interactive and diagnostically important WinUI controls that are part of automated tests use explicit `AutomationProperties.AutomationId` values.
- Automation IDs are stable machine-facing identifiers and are treated as a compatibility surface for tests. Renaming or removing one requires updating and reviewing the affected automation contract.
- FlaUI selectors should prefer automation properties over visible text, screen position, or layout. Visible text selectors are appropriate when the text itself is the behavior being validated.
- Each generated optional-install `ListViewItem` uses the protocol-level `ItemName` as its stable automation ID and the human-facing `DisplayName` as its accessible name. Automation should scope to that item container before locating repeated child controls.
- Repeated controls inside an item template use stable IDs such as `InstallButton` and `RemoveButton`; do not generate their IDs from changing item data.
- Automation IDs do not replace accessibility metadata. Controls should continue to expose meaningful accessible names, roles, focus/keyboard behavior, and state/value semantics.
- The current home automation surface includes `HomeHeading`, `ServiceWarning`, `ItemsList`, item containers identified by `ItemName`, `ItemStatus`, `InstallButton`, and `RemoveButton`. Add similarly intentional IDs as new interactive or diagnostically important UI is introduced.

Signed package workflow (Windows VMs):
- Build VM:
  - Run from repo root (`gorilla/`).
  - `pwsh -File gorilla-ui/tools/build-signed-msix.ps1`
  - Default output directory: `build/` (repo root)
  - Outputs:
    - `build/Gorilla.UI.App.signed.msix`
    - `build/Gorilla.UI.App.cer`
    - `build/win-build.log`
- Target VM (Admin PowerShell):
  - Run from repo root (`gorilla/`) when using default paths.
  - `pwsh -File gorilla-ui/tools/install-signed-msix.ps1`
  - Default input/output directory: `build/` (repo root relative to script location)
  - Handles `already installed` (`0x80073CFB`) by removing the existing package identity and retrying once.
  - Output:
    - `build/win-install.log`

Diagnostics strategy:
- Quiet-by-default behavior:
  - UI client diagnostics are off by default and produce no diagnostics directory/file until explicitly enabled.
  - Service named-pipe trace logs are debug-only (`debug: true` or `--debug`).
  - Baseline Gorilla process logs remain enabled in `gorilla.log` for troubleshooting (both service mode and CLI mode), with console chatter gated by `verbose: true` or `--verbose`.
- Enablement:
  - UI client diagnostics: set `GORILLA_UI_DEBUG=1` (or `GORILLA_DEBUG=1`) before launching Gorilla.UI.App.
  - Service trace diagnostics: set `debug: true` in config or launch Gorilla with `--debug`.
  - Service console verbosity: set `verbose: true` in config or launch Gorilla with `--verbose`.
- Log locations:
  - UI client (Windows runtime): `%LOCALAPPDATA%\\gorilla\\ui-client.log`.
  - Gorilla process log (service mode and CLI mode): `<app_data_path>/gorilla.log` (default `%ProgramData%\\gorilla\\gorilla.log`).
- Retention/rotation policy (implementation target):
  - Cap each log at `10 MiB`.
  - Keep one rotated backup (`*.log.1`).
  - Run cleanup at startup and before first append past the cap.
  - If cleanup fails, continue app/service behavior and keep logging best-effort.
- Required correlation fields for troubleshooting:
  - `requestId`, `operationId`, `operation`, `state`, `result`, `durationMs`.
  - Scope note: required for protocol/operation lifecycle logs; not required for every generic line.

Planned scope for the first release:
- Display available option installs
- Allow install/remove actions
- Show install/remove status updates
