![Gorilla logo](gorilla.png)
# Gorilla [![Build status](https://github.com/1dustindavis/gorilla/actions/workflows/go-test.yml/badge.svg?branch=main)](https://github.com/1dustindavis/gorilla/actions/workflows/go-test.yml)

Munki-like Application Management for Windows

Gorilla is intended to provide application management on Windows using [Munki](https://github.com/munki/munki) as inspiration.
Gorilla supports `.msi`, `.ps1`, `.exe`, or `.nupkg` [(via chocolatey)](https://github.com/chocolatey/choco).

## Getting Started
Information related to installing and configuring Gorilla can be found on the [Wiki](https://github.com/1dustindavis/gorilla/wiki).
For quick manual-test setup helpers on a fresh Windows VM, see [utils/manual-test/README.md](utils/manual-test/README.md).

The official Windows installer is `Gorilla.UI.App.msix` from the releases page. The MSIX requires Windows 10 version 2004 (build 19041) or newer and administrator approval at install time because it installs Gorilla's automatic `LocalSystem` Windows service alongside the WinUI application. The UI itself runs as the signed-in user and communicates with the service over Gorilla's named pipe.

Deploy Gorilla's service configuration at `%ProgramData%\gorilla\config.yaml`. The service intentionally keeps mutable configuration outside the immutable MSIX package. Environments that need custom deployment behavior can use the separately published `gorilla.exe` binary instead of the packaged installer.

## Building

If you just want the latest version, download it from the [releases page](https://github.com/1dustindavis/gorilla/releases).

Building from source requires the [Go tools](https://golang.org/doc/install).

#### macOS and Linux
After cloning this repo, just run `make build`. A new binary will be created in `build/`

#### Windows
After cloning this repo, just run `go build -i ./cmd/gorilla`. A new binary will be created in the current directory.

## Contributing

Pull requests are welcome. The Makefile provides the same validation entry points used by CI:

```text
make verify
    Portable Go + .NET validation expected before pushing.

make verify-windows
    Windows build and source integration validation.

make verify-e2e
    Source-built Gorilla service + unpackaged UI + deterministic fixtures + critical FlaUI workflows.

make verify-release GORILLA_RELEASE_EXE=<path> GORILLA_RELEASE_MSIX=<path>
    Lower validation plus released-binary fixture coverage and full installed-product validation of the produced MSIX/service/UI artifacts.
```

`verify-release` requires a disposable Windows machine with no existing Gorilla package, service, or production configuration. Release CI uses the same deterministic fixture and FlaUI workflows as source E2E, then additionally proves package installation, service registration, named-pipe communication, packaged UI behavior, and uninstall cleanup.

Use smaller targets such as `make go-test`, `make go-staticcheck`, `make ui-lint`, `make ui-test`, `make release-integration`, or `make installed-product-integration` while iterating. See `AGENTS.md` and `make help` for the complete validation contract and guidance on which level applies to a change.

## Repo Admin Mode
Gorilla also supports local repo admin workflows:
- `-b` / `-build`: compile `packages-info/*.yaml` files into catalog files under `catalogs/`
- `-i` / `-import`: scaffold package-info data from an installer (currently stubbed as not yet implemented)

For these modes, set `repo_path` in config (or run from your repo root so the current working directory is used).
See `examples/example_package-info.yaml` for a package-info example.
