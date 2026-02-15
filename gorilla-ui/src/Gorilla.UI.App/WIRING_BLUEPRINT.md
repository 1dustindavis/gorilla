# Gorilla UI App Wiring Blueprint (v0)

This file defines the first implementation pass for wiring WinUI to `Gorilla.UI.Client`.

## Objective
- Use cache-first startup:
  1. load cached `ListOptionalInstalls` data
  2. render immediately
  3. refresh from service
  4. update UI + cache
- Support action flows:
  - `InstallItem`
  - `RemoveItem`
  - `StreamOperationStatus`

## Recommended App Structure
- `App.xaml.cs`
  - Build app services and assign root page.
- `Services/GorillaUiServices.cs`
  - Compose shared client/cache services.
- `Services/OperationTracker.cs`
  - Track active operation streams and emit UI updates.
- `ViewModels/HomeViewModel.cs`
  - Cache-first load and refresh flow.
- `ViewModels/ActivityViewModel.cs`
  - Active/recent operation timeline.
- `Models/UiOptionalInstallItem.cs`
  - UI-focused projection from `OptionalInstallItem`.
- `Views/HomePage.xaml`
  - Store-style cards + primary install/remove action.
- `Views/ActivityPage.xaml`
  - Streamed status lines and operation state.

## Service Registration
Register in app startup (`App.xaml.cs`):

1. `NamedPipeClientOptions` (pipe name/timeouts).
2. `IGorillaServiceClient` -> `NamedPipeGorillaServiceClient`.
3. `IOptionalInstallsCacheStore` -> `JsonFileOptionalInstallsCacheStore`.
4. `OptionalInstallsCacheCoordinator`.
5. View models (`HomeViewModel`, `ActivityViewModel`).

Cache path recommendation:
- `%LOCALAPPDATA%\\Gorilla\\ui\\optional-installs-cache.json`

## HomeViewModel Startup Flow
On page load:
1. `LoadCachedAsync()`
2. If cache exists, project to `UiOptionalInstallItem` and render.
3. Fire `RefreshAsync()` immediately (non-blocking for first paint).
4. Replace or merge list with fresh results.
5. If refresh fails:
   - keep cached data displayed
   - set non-blocking warning banner (service unavailable/stale)

## Item Action Flow
Install:
1. `InstallItemAsync(itemName)` -> receive `operationId`.
2. Mark UI item status as pending.
3. Start `StreamOperationStatusAsync(operationId)` in background.
4. Update item + activity log per event until terminal.

Remove:
1. `RemoveItemAsync(itemName)` -> receive `operationId`.
2. Mark UI item status as pending.
3. Stream status with same flow as install.

## Error Handling
- Cache read/write failure:
  - log warning
  - do not block UI.
- Pipe/service unavailable:
  - show banner and retry action.
- Protocol mismatch:
  - show hard failure message for incompatibility.
- Stream ends before terminal state:
  - mark operation failed (`stream_ended`).

## First Windows Implementation Checklist
1. Run `pwsh -File gorilla-ui/tools/scaffold-winui.ps1` on Windows VM.
2. Add files/folders above into generated app project.
3. Wire `HomePage` to `HomeViewModel`.
4. Add simple action buttons and status text first.
5. Validate with local service + `PipeHarness` side by side.
