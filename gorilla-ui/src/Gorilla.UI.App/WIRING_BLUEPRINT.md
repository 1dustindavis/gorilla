# Gorilla UI App Wiring Blueprint

This document describes the current composition boundary between the committed WinUI app and the portable UI layers.

## Project ownership
- `Gorilla.UI.App`
  - XAML and WinUI lifecycle
  - page/code-behind adapters
  - Windows runtime paths
  - application composition in `App.xaml.cs`
- `Gorilla.UI.Core`
  - `HomeViewModel`
  - `UiOptionalInstallItem`
  - `OperationTracker`
  - cache-first startup orchestration
  - cache abstractions and platform-neutral JSON persistence
- `Gorilla.UI.Client`
  - named-pipe contracts and transport
  - protocol serialization/validation
  - client diagnostics

## Startup composition
`App.xaml.cs` owns concrete runtime construction:

1. Choose the cache path under `%LOCALAPPDATA%\Gorilla\ui`.
2. Construct `NamedPipeGorillaServiceClient`.
3. Construct `JsonFileOptionalInstallsCacheStore`.
4. Construct `OptionalInstallsCacheCoordinator`.
5. Construct `OperationTracker`.
6. Construct `HomeViewModel`.
7. Construct `HomePage` and activate the main window.

The App project may reference Client directly for this composition work, but application/presentation behavior must remain in Core.

## Cache-first startup
`HomeViewModel.InitializeAsync` delegates to Core startup orchestration:

1. Load cached `ListOptionalInstalls` data.
2. Apply cached items immediately when available.
3. Refresh from the service.
4. Replace the visible list with fresh data and update the cache.
5. If refresh fails, retain cached data and expose a non-blocking warning.

## Install/remove flow
1. App forwards the user action to `HomeViewModel`.
2. Core calls `InstallItemAsync` or `RemoveItemAsync` through `IGorillaServiceClient`.
3. Core tracks `StreamOperationStatusAsync` through `OperationTracker`.
4. Core updates presentation state until the stream completes.

The known post-terminal-state list refresh is intentionally not part of Stage 2 and should be implemented as a separate behavior change.

## Rules
- Do not add App-local copies of Core presentation classes.
- Do not add WinUI dependencies to Core.
- Do not move application orchestration into Client.
- Do not regenerate the committed App directory from starter templates.
