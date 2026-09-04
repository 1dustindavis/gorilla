# Gorilla documentation

Gorilla manages application installation, updates, and removal on Windows using manifests and catalogs hosted on a web server.

## Getting started

1. Host `manifests/`, `catalogs/`, and `packages/` directories on an HTTPS server:

   ```text
   web-root/
   ├── manifests/*.yaml
   ├── catalogs/*.yaml
   └── packages/*.{nupkg,msi,exe,ps1,msix}
   ```

2. Create `%ProgramData%\gorilla\config.yaml` with at least `url` and `manifest`. See [Client configuration](client-configuration.md).
3. Install the latest MSIX from [Gorilla releases](https://github.com/1dustindavis/gorilla/releases).

The MSIX requires Windows 10 2004 / build 19041 or newer. It installs the App Catalog UI and registers Gorilla as an automatic `LocalSystem` service. The standalone EXE is available for custom deployments.

By default, logs are written to `%ProgramData%\gorilla\gorilla.log` and the run report to `%ProgramData%\gorilla\GorillaReport.json`.

## Reference

- [Client configuration](client-configuration.md)
- [Windows service](windows-service.md)
- [Catalogs](catalogs.md)
- [Manifests](manifests.md)
- [App Catalog](app-catalog.md)
- [Repository administration](repo-admin-tools.md)
- [Installing Chocolatey with Gorilla](installing-chocolatey.md)
- [Community](community.md)
