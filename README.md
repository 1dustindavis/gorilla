# Gorilla
Munki-like Application Management for Windows

## Overview
Gorilla is intended to provide application management on Windows using [Munki](https://github.com/munki/munki) as inspiration.
Gorilla currently uses [Chocolatey](https://github.com/chocolatey/choco) to install software.

## Config
A configuration file in json format must be passed like this: `gorilla.exe -config <path to config>`.

```json
{
  "url": "https://YourWebServer/gorilla/",
  "manifest": "example",
  "catalog": "production",
  "cachepath": "C:/gorilla/cache"
}
```

* `url` is the directory that includes all of the manifests.
* `manifest` is the manifest that is assigned to this machine.
* `cachepath` is Gorilla's working directory, where we will store copies of manifests. If `cachepath` is not provided, it will default to `%ProgramData%/gorilla/cache`

## Manifests
A manifest can include managed_installs, managed_uninstalls, managed_updates, or additional manifests. Manifests are in json format and must include the name of the manifest:

```json
{
  "name": "example",
  "managed_installs": [ "googlechrome", "slack" ],
  "managed_uninstalls": [ "firefox" ],
  "managed_upgrades": [ "jre8" ],
  "included_manifests": [ "printers", "internal" ]
}
```
## Catalogs
A catalog contains details on all available packages. Catalogs are in json format with each package reperesented by the package name with a nested object containing the package details:

```json
{
	"googlechrome": {
		"display_name": "Google Chrome",
		"installer_item_location": "packages/GoogleChrome.65.0.3325.18100.nupkg",
		"version": "65.0.3325.18100"
	},
	"colorprinter": {
		"display_name": "Color Printer",
		"installer_item_location": "packages/colorprinter.1.0.nupkg",
		"version": "1.0",
		"dependencies": ["Canon-Drivers"]
	},
	"Canon-Drivers": {
		"display_name": "Canon Printer Drivers",
		"installer_item_location": "packages/Canon-Drivers.1.0.nupkg",
		"version": "1.0"
	}
}
```

* `display_name` is currently unused, but optionally includes a human-readable name.
* `installer_item_location` is required for installation and should be the path to the package, relative to the `url` provided in the configuration file.
* `version` is currently unused, but we plan to use this for comparing the current install status.
* `dependencies` is an optional array of package names that should be installed before this package.
