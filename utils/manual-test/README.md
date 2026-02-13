# Manual Test Utils

This directory supports a fast macOS -> Windows VM manual test loop.

## Loop
1. Make code changes on macOS.
2. Run `make bootstrap-run` on macOS.
3. Start the local test server on macOS (included in `bootstrap-run`).
4. Copy generated VM scripts from `build/manual-test/vm/` to the VM.
5. Run one VM bootstrap script to pull the latest binary/config.
6. Run Gorilla manually on the VM.

## 1) Prepare assets on macOS
From repo root:

```bash
make bootstrap-run
```

Or if you want separate steps:

```bash
make bootstrap
./build/manual-test-server -root build/manual-test/server-root -addr :8080
```

This creates:
- `build/manual-test/server-root/gorilla.exe`
- `build/manual-test/server-root/manifests/example_manifest.yaml`
- `build/manual-test/server-root/catalogs/example_catalog.yaml`
- `build/manual-test/server-root/packages/` (empty)
- `build/manual-test-server` (Go static file server)
- `build/manual-test/vm/bootstrap-vm.ps1`
- `build/manual-test/vm/bootstrap-vm.bat` (URL stamped automatically)
- `build/manual-test/vm/bootstrap-vm-full.ps1`
- `build/manual-test/vm/bootstrap-vm-full.bat` (URL stamped automatically)
- `build/manual-test/vm/run-gorilla-check.bat`
- `build/manual-test/vm/base-url.txt` (resolved URL used for stamping)

`make bootstrap` auto-detects a URL like `http://<your-mac-ip>:8080/`.
To override:

```bash
make bootstrap MANUAL_TEST_BASE_URL=http://192.168.1.50:8080/
```

Server source lives in `utils/manual-test/server` (separate Go module).

## 2) Serve assets from macOS

```bash
./build/manual-test-server -root build/manual-test/server-root -addr :8080
```

Use your Mac's reachable IP in the VM, for example:
- `http://192.168.1.50:8080/`

## 3) Bootstrap from the Windows VM
From the copied `build/manual-test/vm/` folder on the VM:

```bat
.\bootstrap-vm.bat
```

Optional switches:
- `.\bootstrap-vm.bat -InstallService -StartService`
- One-off URL override: `.\bootstrap-vm.bat http://192.168.1.99:8080/`

For full integration-test prerequisites (go/chocolatey/WiX):

```bat
.\bootstrap-vm-full.bat
```

Optional switches:
- `.\bootstrap-vm-full.bat -InstallService -StartService`
- One-off URL override: `.\bootstrap-vm-full.bat http://192.168.1.99:8080/`

## 4) Manual run on VM

```bat
.\run-gorilla-check.bat
```

`-C` runs check-only mode so you can quickly validate config/flow without installing packages.

## 5) Run release integration script from VM

From repo root on the VM:

```bat
.\utils\manual-test\run-release-integration.bat
```

Recommended flow first:

```bat
.\build\manual-test\vm\bootstrap-vm-full.bat
```

Optional args:
- `.\utils\manual-test\run-release-integration.bat C:\path\to\gorilla.exe`
- `.\utils\manual-test\run-release-integration.bat C:\path\to\gorilla.exe C:\temp\gorilla-release-integration`
