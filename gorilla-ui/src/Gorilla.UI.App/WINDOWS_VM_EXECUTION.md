# Windows VM Execution Checklist

Use this checklist to complete the remaining WinUI scaffolding/wiring step on Windows.

## 1. Scaffold/refresh WinUI app project
Run from repo root in Developer PowerShell:

```powershell
pwsh -File gorilla-ui/tools/scaffold-winui.ps1
```

Expected result:
- `gorilla-ui/src/Gorilla.UI.App/Gorilla.UI.App.csproj` exists.
- `gorilla-ui/src/Gorilla.UI.App/Gorilla.UI.App.csproj` references `gorilla-ui/src/Gorilla.UI.Client/Gorilla.UI.Client.csproj`.
- `gorilla-ui/Gorilla.UI.slnx` includes `Gorilla.UI.App`.

## 2. Apply starter wiring templates
Copy template files into the generated app project (overwrite placeholders as needed):

- `gorilla-ui/src/Gorilla.UI.App/template/Models/UiOptionalInstallItem.cs`
- `gorilla-ui/src/Gorilla.UI.App/template/Services/GorillaUiServices.cs`
- `gorilla-ui/src/Gorilla.UI.App/template/Services/OperationTracker.cs`
- `gorilla-ui/src/Gorilla.UI.App/template/ViewModels/HomeViewModel.cs`
- `gorilla-ui/src/Gorilla.UI.App/template/Views/HomePage.xaml`
- `gorilla-ui/src/Gorilla.UI.App/template/Views/HomePage.xaml.cs`

## 3. Wire startup composition in app project
In generated `App.xaml.cs`:
- construct `NamedPipeGorillaServiceClient`
- construct `JsonFileOptionalInstallsCacheStore`
- construct `OptionalInstallsCacheCoordinator`
- construct `OperationTracker`
- construct `HomeViewModel`
- set startup page to `HomePage` with injected `HomeViewModel`

Recommended cache path:
- `%LOCALAPPDATA%\\Gorilla\\ui\\optional-installs-cache.json`

## 4. Build and test
Run from repo root:

```powershell
make ui-test
```

Then build solution:

```powershell
dotnet build gorilla-ui/Gorilla.UI.slnx
```

## 5. Basic runtime smoke check
- Start Gorilla service on VM.
- Launch WinUI app.
- Confirm startup behavior:
  - loads cached list if present
  - immediately refreshes from service
- Confirm actions:
  - install/remove issues request
  - operationId returned
  - status stream updates until terminal state
