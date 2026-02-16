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

Debug diagnostics (optional):
- UI client pipe diagnostics are off by default.
- Enable UI client diagnostics by setting `GORILLA_UI_DEBUG=1` (or `GORILLA_DEBUG=1`) before launching Gorilla.UI.App.
- Service named-pipe trace logging is gated by Gorilla config debug mode (`debug: true` or `--debug`).

Planned scope for the first release:
- Display available option installs
- Allow install/remove actions
- Show install/remove status updates
