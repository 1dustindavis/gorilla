# Quick start: your first App Catalog item

This walkthrough publishes one small **demonstration** application item and makes it available as an optional install in App Catalog. The installer creates a marker file rather than installing third-party software. Use a disposable Windows test machine and a repository hosted over HTTPS.

## 1. Prepare the repository

Create the following structure in the web root served at `https://example.com/gorilla/` (replace this URL with your own):

```text
web-root/
├── manifests/
│   └── demo.yaml
├── catalogs/
│   └── demo.yaml
└── packages/
    └── demo-install.ps1
```

Save this as `packages/demo-install.ps1`:

```powershell
$directory = Join-Path $env:ProgramData 'GorillaQuickStart'
New-Item -ItemType Directory -Path $directory -Force | Out-Null
Set-Content -LiteralPath (Join-Path $directory 'installed.txt') -Value '1.0'
```

Calculate its SHA-256 hash **after saving the file**:

```powershell
(Get-FileHash -Algorithm SHA256 .\packages\demo-install.ps1).Hash.ToLowerInvariant()
```

Save `catalogs/demo.yaml`, replacing `REPLACE_WITH_SHA256` with the resulting 64-character hash:

```yaml
QuickStartDemo:
  display_name: Gorilla Quick Start Demo
  description: Creates a demonstration marker file.
  version: 1.0
  check:
    file:
      - path: C:\ProgramData\GorillaQuickStart\installed.txt
  installer:
    type: ps1
    location: packages/demo-install.ps1
    hash: REPLACE_WITH_SHA256
```

Save `manifests/demo.yaml`:

```yaml
name: demo
optional_installs:
  - QuickStartDemo
catalogs:
  - demo
```

Item names must match exactly across the manifest and catalog. Confirm all three files are reachable at the corresponding URLs under your HTTPS web root. This example does not configure removal; it demonstrates the optional **Install** path.

## 2. Configure the Windows client

Create `C:\ProgramData\gorilla\config.yaml` (create the directory first if needed):

```yaml
url: https://example.com/gorilla/
manifest: demo
```

Replace the URL with your repository base URL, **including its trailing slash**. The manifest adds the `demo` catalog, so it does not need to be listed again in the client configuration. Ensure the Windows machine can reach the repository from the Gorilla service's `LocalSystem` context.

## 3. Install and try App Catalog

1. Download the latest `gorilla-<version>.msix` from [GitHub Releases](https://github.com/1dustindavis/gorilla/releases). The MSIX requires Windows 10 build 19041 or newer.
2. Install the MSIX using the normal Windows package installation flow (or `Add-AppxPackage -Path .\gorilla-<version>.msix` in PowerShell, replacing the placeholder with the downloaded filename).
3. Open **App Catalog**, allow its service-backed catalog to refresh, and find **Gorilla Quick Start Demo**.
4. Select **Install**. The item should show as installed after the script creates `C:\ProgramData\GorillaQuickStart\installed.txt`.

If the item does not appear, confirm the URLs and YAML item names, use the App Catalog refresh action, and check `C:\ProgramData\gorilla\gorilla.log`. A stale or unavailable service may prevent refresh and actions.

For production software, use a real installer and appropriate status checks, hashes, and (if removal is desired) uninstaller metadata. See [Catalogs](catalogs.md), [Manifests](manifests.md), [Client configuration](client-configuration.md), and [App Catalog](app-catalog.md).
