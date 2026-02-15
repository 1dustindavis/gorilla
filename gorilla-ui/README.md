# Gorilla UI

This folder contains all UI app related code for the Gorilla WinUI client.

Tooling requirements:
- macOS development: `dotnet-sdk@8`
- Windows VM development: Visual Studio 2022 with WinUI/Windows App SDK tooling and .NET 8 SDK

Current workspace:
- Solution file: `gorilla-ui/Gorilla.UI.slnx`
- Included projects:
  - `gorilla-ui/src/Gorilla.UI.Client/Gorilla.UI.Client.csproj`
  - `gorilla-ui/tests/Gorilla.UI.Client.Tests/Gorilla.UI.Client.Tests.csproj`
  - `gorilla-ui/tools/PipeHarness/PipeHarness.csproj`

Windows VM scaffold helper:
- `pwsh -File gorilla-ui/tools/scaffold-winui.ps1`
- This scaffolds `gorilla-ui/src/Gorilla.UI.App/Gorilla.UI.App.csproj`, adds a reference to `Gorilla.UI.Client`, and adds the app project to `gorilla-ui/Gorilla.UI.slnx`.

Planned scope for the first release:
- Display available option installs
- Allow install/remove actions
- Show install/remove status updates
