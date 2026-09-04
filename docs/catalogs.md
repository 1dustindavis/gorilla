# Catalogs

A catalog maps unique item names to installation, removal, and status-check metadata. Catalogs are loaded in the order configured; each catalog should contain only one entry for a given item name.

```yaml
ExampleApp:
  display_name: Example App
  version: 1.2.3
  dependencies:
    - ExampleDependency
  check:
    registry:
      name: Example App
      version: 1.2.3
  installer:
    type: msi
    location: packages/example-app-1.2.3.msi
    hash: <sha256>
    arguments:
      - /L=1033
```

See [the complete example catalog](../examples/example_catalog.yaml).

## Item keys

- `display_name`: Human-readable item name.
- `version`: Desired application version.
- `dependencies`: Item names that must be processed first.
- `check`: One status-check definition.
- `installer`: Installation metadata.
- `uninstaller`: Removal metadata for `managed_uninstalls` and optional removals.
- `preinstall_script`: PowerShell run before installation; a nonzero exit stops the install.
- `postinstall_script`: PowerShell run after the installation attempt; a nonzero exit is reported as an error.

## Status checks

Gorilla selects the first configured check in this order: `script`, `file`, `registry`, then `appx`.

### File

```yaml
check:
  file:
    - path: C:\Program Files\Example\example.exe
      version: 1.2.3
      hash: <sha256>
```

`path` is required. `version` and `hash` are optional checks against the file's Windows metadata or content. Multiple file entries may be supplied.

### Script

```yaml
check:
  script: |
    if (Test-Path 'C:\Program Files\Example\example.exe') { exit 1 }
    exit 0
```

For installs and updates, exit `0` means action is needed and a nonzero exit means no action. Removal uses the inverse interpretation.

### Registry

```yaml
check:
  registry:
    name: Example App
    version: 1.2.3
```

`name` is matched against `DisplayName` entries under the Windows uninstall registry keys. `version` is compared with `DisplayVersion`.

### AppX/MSIX

```yaml
check:
  appx:
    name: Example.Package.Name
    version: 1.2.3.0
```

`name` is the package `DisplayName` returned by `Get-AppxProvisionedPackage -Online`; `version` is the minimum acceptable version.

## Installer and uninstaller keys

- `type`: `nupkg`, `msi`, `exe`, `ps1`, or `msix`.
- `location`: Package path relative to `url_packages` or `url`.
- `hash`: Required SHA-256 of the package.
- `arguments`: Optional arguments for MSI, EXE, or MSIX installation and EXE removal.
- `package_id`: Optional Chocolatey package ID for `nupkg`; setting it avoids ambiguous automatic detection.

For MSIX removal, set `uninstaller.type: msix`. No uninstaller `location` or `hash` is needed; Gorilla uses `check.appx.name` to remove the provisioned package and per-user registrations.
