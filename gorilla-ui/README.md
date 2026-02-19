# Gorilla UI

This folder contains all UI app related code for the Gorilla WinUI client.

Tooling requirements:
- macOS development: `dotnet-sdk@8`
- Windows VM development: Visual Studio 2022 with WinUI/Windows App SDK tooling and .NET 8 SDK

Current workspace:
- Solution file: `gorilla-ui/Gorilla.UI.sln`
- Included projects:
  - `gorilla-ui/src/Gorilla.UI.Client/Gorilla.UI.Client.csproj`
  - `gorilla-ui/tests/Gorilla.UI.Client.Tests/Gorilla.UI.Client.Tests.csproj`
  - `gorilla-ui/tools/PipeHarness/PipeHarness.csproj`

Validation commands:
- `make ui-lint` runs `dotnet build -warnaserror` for:
  - `gorilla-ui/src/Gorilla.UI.Client/Gorilla.UI.Client.csproj`
  - `gorilla-ui/tests/Gorilla.UI.Client.Tests/Gorilla.UI.Client.Tests.csproj`
  - `gorilla-ui/tools/PipeHarness/PipeHarness.csproj`
- `make ui-test` runs the Gorilla UI .NET test project.
- Windows UI tests (FlaUI):
  - CI workflow: `.github/workflows/windows-ui-test.yml`
  - Runner: `windows-2025`
  - Schedule: daily at 09:00 UTC
  - Behavior: non-blocking (`continue-on-error: true`) with up to 3 attempts
  - Artifacts: TRX results and failure screenshots/logs from `gorilla-ui/tests/Gorilla.UI.App.WindowsUiTests`
  - Local run (Windows):
    - Build app:
      - `dotnet build gorilla-ui/src/Gorilla.UI.App/Gorilla.UI.App.csproj -c Release -p:Platform=x64`
    - Set app path and run Windows UI tests:
      - `$env:GORILLA_UI_APP_EXE = "<path-to-Gorilla.UI.App.exe>"`
      - `dotnet test gorilla-ui/tests/Gorilla.UI.App.WindowsUiTests/Gorilla.UI.App.WindowsUiTests.csproj -c Release`
- Optional local autofix: `dotnet format gorilla-ui/src/Gorilla.UI.Client/Gorilla.UI.Client.csproj`.

Windows VM scaffold helper:
- `pwsh -File gorilla-ui/tools/scaffold-winui.ps1`
- This scaffolds `gorilla-ui/src/Gorilla.UI.App/Gorilla.UI.App.csproj`, adds a reference to `Gorilla.UI.Client`, and adds the app project to `gorilla-ui/Gorilla.UI.sln`.

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
