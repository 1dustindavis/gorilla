# Windows service

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
