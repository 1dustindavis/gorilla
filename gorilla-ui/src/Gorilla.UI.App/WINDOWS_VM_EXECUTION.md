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
- `gorilla-ui/Gorilla.UI.sln` includes `Gorilla.UI.App`.

## 2. Apply starter wiring templates
Run from repo root:

```powershell
pwsh -File gorilla-ui/tools/apply-winui-templates.ps1
```

This copies template files into the generated app project (overwriting placeholders as needed):

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
dotnet build gorilla-ui/Gorilla.UI.sln
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

## 6. Reproducible cache-first startup validation
Use these exact steps to validate TODO #3 behavior with live service data.

1. Seed cache with a recognizable stale entry.
   - Close Gorilla UI if open.
   - Write `%LOCALAPPDATA%\Gorilla\ui\optional-installs-cache.json` with a single item that does not match the live service list (for example `CachedOnlyApp`).
2. Launch Gorilla UI while service is running.
   - Expected: `CachedOnlyApp` renders first.
   - Expected: list is replaced by live `ListOptionalInstalls` results within the refresh window.
3. Validate stale-data warning path.
   - Stop Gorilla service.
   - Relaunch Gorilla UI with the same cache file still present.
   - Expected: cached items remain visible.
   - Expected: warning banner includes `Showing cached data. Refresh failed:`.
4. Restore service and relaunch UI.
   - Expected: warning clears after successful refresh and list converges to live service data.

Record results in PR notes with:
- VM image/build identifier
- cache fixture payload used
- observed first-render list
- observed post-refresh list
- observed stale-data warning text
