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
  - cache-first startup and snapshot-refresh orchestration
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

1. Load usable cached `ListOptionalInstalls` presentation data.
2. Apply cached items immediately when available.
3. Request `ListOptionalInstalls` with `refresh:true`.
4. Apply an available service snapshot immediately when it is not older than the already-presented trusted cache.
5. If service refresh is queued/running, keep existing data visible, present `Refreshing…`, and poll with `refresh:false`.
6. Apply the newer snapshot when publication completes and update the UI cache.
7. If service access or refresh fails, retain usable cached/snapshot data and expose degraded state rather than discarding presentation.

The service snapshot and UI cache cover different failure domains. The service snapshot is service-owned last-known-good state persisted under ProgramData; the per-user UI cache exists so presentation remains available when the service or pipe is unavailable. Neither is mutation authority.

See [App Catalog snapshot architecture](../../docs/app-catalog-snapshot-architecture.md) for the complete refresh, freshness, persistence, and publication contract.

## Install/remove flow
1. App forwards the user action to `HomeViewModel`.
2. Core calls `InstallItemAsync` or `RemoveItemAsync` through `IGorillaServiceClient`.
3. The service revalidates the requested mutation against current service truth; cached/snapshot actions are presentation only.
4. Core tracks `StreamOperationStatusAsync` through `OperationTracker`.
5. Service-side targeted execution verifies the postcondition and publishes the resulting App Catalog snapshot.
6. After the terminal event, Core reconciles by reading `ListOptionalInstalls` with `refresh:false`; it does not request a redundant full regeneration.

## Manual Refresh
Manual Refresh keeps existing catalog data visible, requests `refresh:true`, presents `Refreshing…`, and polls cheap `refresh:false` reads until refresh becomes `Idle` or `Failed`. A named-pipe request is never held open waiting for catalog projection to finish.

## Rules
- Do not add App-local copies of Core presentation classes.
- Do not add WinUI dependencies to Core.
- Do not move application orchestration into Client.
- Do not regenerate the committed App directory from starter templates.
- Do not use UI cache or service snapshot data to authorize Install/Remove.
