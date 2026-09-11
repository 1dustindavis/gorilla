# Windows service

The [App Catalog contract](../gorilla-ui/docs/app-catalog-contract.md) defines the state, policy, action, and result model used by the live v1 protocol. [App Catalog operation recovery](../gorilla-ui/docs/app-catalog-recovery.md) documents mutation identity, bounded in-memory operation retention, reconnect behavior, and service-restart semantics.

The Gorilla service runs managed application processing on a schedule and exposes a named-pipe endpoint for App Catalog and command-line requests.

- Service name: `gorilla`
- Named pipe: `\\.\pipe\gorilla-service`
- Account: `LocalSystem` for the packaged service
- Configuration: `%ProgramData%\gorilla\config.yaml`
- Local service manifest: `<app_data_path>\service-manifest.yaml`
- Default interval: `1h`

Set another interval with Go duration syntax:

```yaml
service_interval: 30m
```

## Official installation

The versioned `gorilla-<version>.msix` release artifact installs App Catalog and registers the automatic Gorilla service.

## Standalone deployment

The standalone `gorilla-<version>.exe` supports custom service deployment. Run these commands from an elevated PowerShell or Command Prompt:

```powershell
gorilla.exe -c C:\ProgramData\gorilla\config.yaml -serviceinstall
gorilla.exe -c C:\ProgramData\gorilla\config.yaml -servicestart
gorilla.exe -c C:\ProgramData\gorilla\config.yaml -servicestatus
gorilla.exe -c C:\ProgramData\gorilla\config.yaml -servicestop
gorilla.exe -c C:\ProgramData\gorilla\config.yaml -serviceremove
```

Run the service loop in the foreground for troubleshooting:

```powershell
gorilla.exe -c C:\ProgramData\gorilla\config.yaml -service
```

## Service commands

Use `-S` or `-servicecmd` to communicate with the running service:

```powershell
gorilla.exe -S ListOptionalInstalls
gorilla.exe -S InstallItem:VLC
gorilla.exe -S RemoveItem:VLC
gorilla.exe -S StreamOperationStatus:<operationId>
```

Process logs are written to `<app_data_path>\gorilla.log`, which defaults to `%ProgramData%\gorilla\gorilla.log`.

`ListOptionalInstalls` resolves effective optional assignments using manifest and
catalog precedence. Missing or invalid entries remain visible with an unknown
state and disabled actions. Detection failures remain distinct from absence.
Multiple plausible registry substring matches are reported as unknown without
an installed version instead of exposing a map-order-dependent match.
`installRequirement` separately carries the selected check's Satisfied or
NotSatisfied result. This allows script-check items to remain installable without
claiming that the script proved physical presence or a version. Gorilla's
script-first check precedence is unchanged.
`InstallItem` and `RemoveItem` re-resolve policy before changing local selection,
so the same restrictions apply to App Catalog and direct command-line requests.
Install authorization validates the complete dependency graph and rejects
missing or invalid dependencies, cycles, and dependencies assigned to managed
uninstall. Before each scheduled or requested managed run, the service removes
local install selections superseded by administrator-managed uninstall policy.

Installing adds a persistent local managed install. Removing clears that
selection and supplies a service-owned temporary uninstall manifest to the next
serialized managed run. The temporary manifest is removed after the run and is
not added to configured local manifests. Old `managed_uninstalls` written inside
`service-manifest.yaml` are cleared when the service starts; administrator-owned
managed uninstalls in other manifests are unchanged. This automatic migration
prevents an old UI removal request from repeatedly uninstalling software. If the
service-owned manifest cannot be read, parsed, or rewritten, startup stops with
the manifest path, reason for the migration, and underlying filesystem/YAML error.
