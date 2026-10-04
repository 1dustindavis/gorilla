![Gorilla logo](gorilla.png)
[![Build status](https://github.com/1dustindavis/gorilla/actions/workflows/go-test.yml/badge.svg?branch=main)](https://github.com/1dustindavis/gorilla/actions/workflows/go-test.yml)
# Gorilla

Munki-like Application Management for Windows

Gorilla supports `.msi`, `.ps1`, `.exe`, or `.nupkg` [(via chocolatey)](https://github.com/chocolatey/choco).

## Getting Started
See the [Gorilla documentation](docs/README.md).

Download the latest version from the [releases page](https://github.com/1dustindavis/gorilla/releases). MSIX is the recommended installation method.

A UI for Gorilla called [App Catalog](docs/app-catalog.md) is still in development and is included with the MSIX.

## Building

If you just want the latest version, download it from the [releases page](https://github.com/1dustindavis/gorilla/releases).

Building from source requires the [Go tools](https://golang.org/doc/install).

#### macOS and Linux
After cloning this repo, just run `make build`. A new binary will be created in `build/`

#### Windows
After cloning this repo, just run `go build -i ./cmd/gorilla`. A new binary will be created in the current directory.

## Contributing

Pull requests are welcome. Validate changes with:

```text
make verify
```

## Repository administration

Gorilla can deterministically build generated catalogs from package-info records and preview repository cleanup without modifying files:

```text
gorilla admin build
gorilla admin build --repo /path/to/gorilla-repo
gorilla admin cleanup
gorilla admin cleanup --repo /path/to/gorilla-repo --keep 3
```

The repository path defaults to the current working directory. Repository administration does not use normal Gorilla client configuration. Cleanup is dry-run-only and retains three package-info versions per live item by default. See [Repository administration](docs/repo-admin-tools.md) and [`examples/example_package-info.yaml`](examples/example_package-info.yaml).
