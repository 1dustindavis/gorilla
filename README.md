# Gorilla
Munki-like Application Management for Windows

## Overview
Gorilla is intended to provide application management on Windows using [Munki](https://github.com/munki/munki) as inspiration.
Gorilla supports `.msi`, `.ps1`, or `.nupkg` [(via chocolatey)](https://github.com/chocolatey/choco).

All files can be served from any standard web server with a directory stucture like this:

```bash
[web root]
├── manifests
│   ├── *.yaml
├── catalogs
│   ├── *.yaml
└── packages
    ├── *.nupkg
    └── *.msi
```

## Config
A configuration file in yaml format must be passed like this: `gorilla.exe -config <path to config>`.

```yaml
---
url: https://YourWebServer/gorilla/
manifest: example
catalog: production
cachepath: C:/gorilla/cache
```

* `url` is the directory that includes all of the manifests.
* `manifest` is the manifest that is assigned to this machine.
* `cachepath` is Gorilla's working directory, and may store copies of manifests, catalogs, or packages. If `cachepath` is not provided, it will default to `%ProgramData%/gorilla/cache`

## Manifests
A manifest can include managed_installs, managed_uninstalls, managed_updates, or additional manifests. Manifests are in yaml format and must include the name of the manifest:

```yaml
---
name: example
managed_installs:
  - GoogleChrome
  - Slack
managed_uninstalls:
  - Firefox
managed_updates:
  - Jre8
included_manifests:
  - printers
  - internal
```
## Catalogs
A catalog contains details on all available packages. Catalogs are in yaml format with each package reperesented by the package name with a nested object containing the package details:

```yaml
---
GoogleChrome:
  display_name: Google Chrome
  installer_item_hash: c1ed04713c5a8b4ff8bc7d77036644dac505784818b91850f180e08da786fbca
  installer_item_location: packages/GoogleChrome.65.0.3325.18100.msi
  version: 65.0.3325.18100

ColorPrinter:
  display_name: Color Printer
  installer_item_hash: a8b4ff8bc7d77036644c1ed04713c550550f180e08da786fbca784818b918dac
  installer_item_location: packages/colorprinter.1.0.nupkg
  version: 1.0
  dependencies: Canon-Drivers

CanonDrivers:
  display_name: Canon Printer Drivers
  installer_item_hash: ca784818b91850f180e08da786ac1ed04713c5a8b4ff8bc7d77036644dac505aec
  installer_item_location: packages/Canon-Drivers.1.0.nupkg
  version: 1.0

Chocolatey:
  display_name: Chocolatey
  install_check_path: C:\ProgramData\chocolatey\bin\choco.exe
  installer_item_location: packages/apps/chocolatey/chocolateyInstall.ps1
  installer_item_hash: 38cf17a230dbe53efc49f63bbc9931296b5cea84f45ac6528ce60767fe370230
  version: 1.0

ChefClient:
  display_name: Chef Client
  install_check_script: |
    $latest = "13.8.5"
    $current = chef-client --version
    $current = $current.Split(" ")[1]
    $upToDate = [System.Version]$current -gt [System.Version]$latest
    If ($upToDate) {
      exit 1
    } Else {
      exit 0
    }
  installer_item_location: packages/apps/chef-client/chef-client-13.8.5-x64.msi
  installer_item_hash: c1ed04719c5a8b4ff9bc7d77036644dec505984808b91850f180e08da786fbca
  version: 13.8.5

```

* `display_name` should be a human-readable name, and match the Display name of registry items.
* `install_check_path` is a path to a file that must exist for the item to be considered installed. If this option is not provided, Gorilla will default to using the registry.
* `install_check_script` is a PowerShell code block that will be executed to determine if the package should be installed. Any non-zero exit code will be considered installed. If this option is not provided, Gorilla will default to using the registry.
* `installer_item_hash` is required and should be a sha256 hash of the file located at `installer_item_location`.
* `installer_item_location` is required and should be the path to the package, relative to the `url` provided in the configuration file.
* `version` is compared to the currently installed version to determine if it needs to be installed.
* `dependencies` is an optional array of package names that should be installed before this package.
