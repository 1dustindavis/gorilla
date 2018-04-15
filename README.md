# Gorilla
Munki-like Application Management for Windows

## Overview
Gorilla is intended to provide application management on Windows using [Munki](https://github.com/airbnb/gosal) as inspiration.
Gorilla currently uses [Chocolatey](https://github.com/chocolatey/choco) to install software.

## Config
A configuration file in json format must be passed like this: `gorilla.exe -config <path to config>`

```json
{
  "url": "https://YourWebServer/gorilla/",
  "manifest": "example",
  "cachepath": "C:/gorilla/cache"
}
```

* `url` is the directory that includes all of the manifests
* `manifest` is the manifest that is assigned to this machine
* `cachepath` is Gorilla's working directory, where we will store copies of manifests 

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
