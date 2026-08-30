# Windows VM Execution Checklist

The WinUI application is committed source. Do not scaffold or regenerate `gorilla-ui/src/Gorilla.UI.App` before building it.

## Build and validate
From the repository root in Developer PowerShell:

```powershell
make ui-test
make ui-windows-build
```

For the full Windows validation stack:

```powershell
make verify-e2e
```

## Expected project boundary
- `Gorilla.UI.App` references `Gorilla.UI.Core` and `Gorilla.UI.Client`.
- `Gorilla.UI.Core` is ordinary `net8.0` and contains the platform-neutral presentation/workflow behavior.
- `Gorilla.UI.Client` contains named-pipe protocol/client concerns.

## Runtime smoke check
- Start the Gorilla service on the VM.
- Launch Gorilla UI.
- Confirm startup behavior:
  - cached list renders when present
  - fresh service data replaces cached data
  - a refresh failure leaves cached data visible with a warning
- Confirm install/remove behavior:
  - request is accepted or rejected correctly
  - accepted operations stream status updates
  - terminal success/failure/cancellation is reflected in presentation state

## Reproducible cache-first validation
1. Close Gorilla UI.
2. Seed `%LOCALAPPDATA%\Gorilla\ui\optional-installs-cache.json` with a recognizable stale entry.
3. Launch with the service running and confirm the cached entry appears before fresh service data replaces it.
4. Stop the service and relaunch; confirm cached data remains visible and the warning includes `Showing cached data. Refresh failed:`.
5. Restore the service and relaunch; confirm the warning clears after a successful refresh.

Record any manual validation results in the PR notes with the VM image/build identifier and the observed behavior.
