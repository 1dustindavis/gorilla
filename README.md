![Gorilla logo](gorilla.png)
# Gorilla [![Build status](https://github.com/1dustindavis/gorilla/actions/workflows/go-test.yml/badge.svg?branch=main)](https://github.com/1dustindavis/gorilla/actions/workflows/go-test.yml)

Munki-like Application Management for Windows

Gorilla is intended to provide application management on Windows using [Munki](https://github.com/munki/munki) as inspiration.
Gorilla supports `.msi`, `.ps1`, `.exe`, or `.nupkg` [(via chocolatey)](https://github.com/chocolatey/choco).

## Getting Started
Information related to installing and configuring Gorilla can be found on the [Wiki](https://github.com/1dustindavis/gorilla/wiki).
For quick manual-test setup helpers on a fresh Windows VM, see [utils/manual-test/README.md](utils/manual-test/README.md).

Releases include `gorilla-<version>.msix` (Windows 10 2004 / build 19041+) and a standalone `gorilla-<version>.exe`.
Service configuration lives at `%ProgramData%\gorilla\config.yaml`.

## Building

If you just want the latest version, download it from the [releases page](https://github.com/1dustindavis/gorilla/releases).

Building from source requires the [Go tools](https://golang.org/doc/install).

#### macOS and Linux
After cloning this repo, just run `make build`. A new binary will be created in `build/`

#### Windows
After cloning this repo, just run `go build -i ./cmd/gorilla`. A new binary will be created in the current directory.

## Contributing

Pull requests are welcome. Key validation targets:

```text
make verify
make verify-windows
make verify-e2e
make verify-release GORILLA_RELEASE_EXE=<path> GORILLA_RELEASE_MSIX=<path>
```

See `AGENTS.md` and `make help` for details.

## Repo Admin Mode
Gorilla also supports local repo admin workflows:
- `-b` / `-build`: compile `packages-info/*.yaml` files into catalog files under `catalogs/`
- `-i` / `-import`: scaffold package-info data from an installer (currently stubbed as not yet implemented)

For these modes, set `repo_path` in config (or run from your repo root so the current working directory is used).
See `examples/example_package-info.yaml` for a package-info example.
