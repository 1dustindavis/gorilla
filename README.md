# Gorilla

[![Go Report Card](https://goreportcard.com/badge/github.com/1dustindavis/gorilla)](https://goreportcard.com/report/github.com/1dustindavis/gorilla) [![Build status](https://ci.appveyor.com/api/projects/status/hvug2p5wsvlor2v0/branch/master?svg=true)](https://ci.appveyor.com/project/DustinDavis/gorilla/branch/master)

Munki-like Application Management for Windows

**[Getting Started](https://github.com/1dustindavis/gorilla/wiki)**

## Overview
Gorilla is intended to provide application management on Windows using [Munki](https://github.com/munki/munki) as inspiration.
Gorilla supports `.msi`, `.ps1`, `.exe`, or `.nupkg` [(via chocolatey)](https://github.com/chocolatey/choco).

All files can be served from any standard web server with a directory stucture like this:

```bash
[web root]
├── manifests
│   ├── *.yaml
├── catalogs
│   ├── *.yaml
└── packages
    ├── *.nupkg
    ├── *.msi
    ├── *.exe
    └── *.ps1
```

## Config
The configuration file is in yaml format and defaults to `%ProgramData%/gorilla/config.yaml`, but alternatively may be passed like this: `gorilla.exe -config <path to config>`.

```yaml
---
url: https://YourWebServer/gorilla/
manifest: example
catalogs: 
  - alpha
  - beta
cachepath: C:/gorilla/cache
auth_user: GorillaRepoUser
auth_pass: pizzaisyummy
tls_auth: true
tls_client_cert: c:/certs/client.pem
tls_client_key: c:/certs/client.key
tls_server_cert: c:/certs/server.pem
```

### Required Keys
* `url` is the path on your server that contains the directories for manifests, catalogs, and packages.
* `manifest` is the primary manifest that is assigned to this machine.

### Optional Keys
* `catalogs` is an array of catalogs that are assigned to this machine. If you do not provide a catalog in the config, you must have one in a manifest.
* `app_data_path` is Gorilla's working directory, and may store copies of manifests, catalogs, or packages. If `app_data_path` is not provided, it will default to `%ProgramData%/gorilla/`.
* `url_packages` is an optional base url to be used instead of `url` when downloading packages.

Basic Auth
* `auth_user` is an optional username for http basic auth.
* `auth_pass` is an option password for http basic auth.

TLS Auth
* `tls_auth` must be true if you are using TLS mutual authentication.
* `tls_client_cert` is the absolute path to your client certificate in PEM format.
* `tls_client_key` is the absolute path to your client private key in PEM format.
* `tls_server_cert` is the absolute path to your server's CA cert in PEM format.

## Manifests
A manifest can include managed_installs, managed_uninstalls, managed_updates, or additional manifests. Catalogs can also be assigned via manifests. Manifests are in yaml format and must include the name of the manifest:

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
catalogs:
  - production
```
## Catalogs
A catalog contains details on all available packages. Catalogs are in yaml format with each package reperesented by the package name with a nested object containing the package details:

```yaml
---
GoogleChrome:
  display_name: Google Chrome
  installer_item_hash: ce9c44417489d6c1f205422a4b9e8d5181d1ac24b6dcae3bd68ec315efdeb18b
  installer_item_location: packages/google-chrome/GoogleChrome.68.0.3440.106.nupkg
  installer_type: nupkg
  version: 68.0.3440.106

ColorPrinter:
  dependencies: Canon-Drivers
  display_name: Color Printer
  installer_item_hash: a8b4ff8bc7d77036644c1ed04713c550550f180e08da786fbca784818b918dac
  installer_item_location: packages/colorprinter.1.0.nupkg
  installer_type: nupkg
  version: 1.0

CanonDrivers:
  display_name: Canon Printer Drivers
  installer_item_hash: ca784818b91850f180e08da786ac1ed04713c5a8b4ff8bc7d77036644dac505aec
  installer_item_location: packages/Canon-Drivers.1.0.nupkg
  installer_type: nupkg
  version: 1.0

Chocolatey:
  display_name: Chocolatey
  install_check_path: C:\ProgramData\chocolatey\bin\choco.exe
  installer_item_location: packages/chocolatey/chocolateyInstall.ps1
  installer_item_hash: 38cf17a230dbe53efc49f63bbc9931296b5cea84f45ac6528ce60767fe370230
  installer_type: ps1
  version: 1.0

ChefClient:
  display_name: Chef Client
  install_check_script: |
    $latest = "14.3.37"
    $chefPath = "C:\opscode\chef\bin\chef-client.bat"
    If (![System.IO.File]::Exists($chefPath)) {
      exit 0
    }
    $current = C:\opscode\chef\bin\chef-client.bat --version
    $current = $current.Split(" ")[1]
    $upToDate = [System.Version]$current -ge [System.Version]$latest
    If ($upToDate) {
      exit 1
    } Else {
      exit 0
    }
  installer_item_location: packages/chef-client/chef-client-14.3.37-1-x64.msi
  installer_item_hash: f5ef8c31898592824751ec2252fe317c0f667db25ac40452710c8ccf35a1b28d
  installer_type: msi
  uninstaller_item_location: packages/chef-client/chef-client-14.3.37-1-x64.msi
  uninstaller_item_hash: f5ef8c31898592824751ec2252fe317c0f667db25ac40452710c8ccf35a1b28d
  uninstaller_type: msi
  version: 14.3.37

vlc:
  display_name: VLC
  install_check_path: C:\Program Files (x86)\VideoLAN\VLC\vlc.exe
  install_check_path_hash: 86376d909ab4ff020a9b0477f17efeee736cf1eb2020ded3c511188f8571ebc5
  installer_item_location: packages/apps/vlc/vlc-3.0.3-win32.exe
  installer_item_hash: 65bf42b15a05b13197e4dd6cdf181e39f30d47feb2cb6cc929db21cd634cd36f
  installer_item_arguments: 
     - /L=1033
     - /S
  installer_type: exe
  uninstaller_item_location: packages/apps/vlc/vlc-3.0.3-uninstall.exe
  uninstaller_item_hash: 676dcb69da99728feb8af3231e863dbb9639dc09f409749a74dd5c08dc2fb809
  uninstaller_item_arguments: 
     - /S
  uninstaller_type: exe
  version: 3.0.3

```

* `dependencies` is an optional array of package names that should be installed before this package.
* `display_name` should be a human-readable name, and match the Display name of registry items.
* `install_check_path` is a path to a file that must exist for the item to be considered installed. If this option is not provided, Gorilla will default to using the registry.
* `install_check_path_hash` is an optional sha256 of the item that is expected at `install_check_path`.
* `install_check_script` is a PowerShell code block that will be executed to determine if the package should be installed. Any non-zero exit code will be considered installed. If this option is not provided, Gorilla will default to using the registry.
* `installer_item_arguments` is an optional list of arguments to pass to the installer. Currently only supported by exe installers.
* `installer_item_hash` is a **required** sha256 hash of the file located at `installer_item_location`.
* `installer_item_location` is **required** and should be the path to the package, relative to the `url` provided in the configuration file.
* `installer_type` is **required** type of installer located at `installer_item_location` and can be `nupkg`, `msi`, `exe`, or `ps1`.
* `uninstaller_item_arguments` same as `installer_item_arguments`, but used when the item is configured as a `managed_uninstall`.
* `uninstaller_item_hash` same as `installer_item_hash`, but used when the item is configured as a `managed_uninstall`.
* `uninstaller_item_location` same as `installer_item_location`, but used when the item is configured as a `managed_uninstall`.
* `uninstaller_type` same as `installer_type`, but used when the item is configured as a `managed_uninstall`.
* `version` is compared to the currently installed version to determine if it needs to be installed (currently only support the registry)
