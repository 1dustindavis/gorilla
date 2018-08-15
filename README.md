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
* `cachepath` is Gorilla's working directory, where we will store copies of manifests. If `cachepath` is not provided, it will default to `%ProgramData%/gorilla/cache`

## Manifests
A manifest can include managed_installs, managed_uninstalls, managed_updates, or additional manifests. Manifests are in yaml format and must include the name of the manifest:

```yaml
---
name: example
managed_installs:
  - googlechrome
  - slack
managed_uninstalls:
  - firefox
managed_upgrades:
  - jre8
included_manifests:
  - printers
  - internal
```
## Catalogs
A catalog contains details on all available packages. Catalogs are in yaml format with each package reperesented by the package name with a nested object containing the package details:

```yaml
---
googlechrome:
  display_name: Google Chrome
  installer_item_hash: c1ed04713c5a8b4ff8bc7d77036644dac505784818b91850f180e08da786fbca
  installer_item_location: packages/GoogleChrome.65.0.3325.18100.msi
  version: 65.0.3325.18100

colorprinter:
  display_name: Color Printer
  installer_item_hash: a8b4ff8bc7d77036644c1ed04713c550550f180e08da786fbca784818b918dac
  installer_item_location: packages/colorprinter.1.0.nupkg
  version: 1.0
  dependencies: Canon-Drivers

Canon-Drivers:
  display_name: Canon Printer Drivers
  installer_item_hash: ca784818b91850f180e08da786ac1ed04713c5a8b4ff8bc7d77036644dac505aec
  installer_item_location: packages/Canon-Drivers.1.0.nupkg
  version: 1.0
```

* `display_name` is currently unused, but optionally includes a human-readable name.
* `installer_item_hash` is required and should be a sha256 hash of the file located at `installer_item_location`.
* `installer_item_location` is required and should be the path to the package, relative to the `url` provided in the configuration file.
* `version` is compared to the currently installed version to determine if it needs to be installed.
* `dependencies` is an optional array of package names that should be installed before this package.
