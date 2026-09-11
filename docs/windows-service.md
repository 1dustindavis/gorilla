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
