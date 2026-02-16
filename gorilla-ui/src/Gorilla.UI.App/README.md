# Gorilla.UI.App

WinUI 3 application project for Gorilla UI.

## Notes
- This project should be scaffolded on a Windows VM using WinUI 3 templates.
- Keep app-specific code here (views, view models, app composition).
- Consume IPC through `Gorilla.UI.Client` abstractions.

## Intended commands (Windows)
- `dotnet new sln -n Gorilla.UI`
- `dotnet new winui3 -n Gorilla.UI.App`
- Add the app project under this folder and reference `../Gorilla.UI.Client`.

## Wiring templates
- Starter wiring templates are available under `gorilla-ui/src/Gorilla.UI.App/template/`.
- These include:
  - cache-first home viewmodel flow
  - named-pipe service composition
  - operation status tracker stub
  - starter Home page XAML/code-behind
- Windows helper to apply templates into generated app project:
  - `pwsh -File gorilla-ui/tools/apply-winui-templates.ps1`
