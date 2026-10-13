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

The primary `make build` target produces the Windows amd64 client binary at `build/gorilla.exe`.

Repository administrators on macOS or Linux can download the native admin binaries from the [releases page](https://github.com/1dustindavis/gorilla/releases). See [Repository administration](docs/repo-admin-tools.md).

## Contributing

Pull requests are welcome. Validate changes with:

```text
make verify
```

## Repository administration

Gorilla includes tools to build catalogs and clean up superseded or abandoned repository content. Cleanup is a dry run unless `--apply` is provided.

```text
gorilla admin build
gorilla admin cleanup
gorilla admin cleanup --keep 3
gorilla admin cleanup --apply
```

The repository path defaults to the current working directory. Downloadable macOS and Linux admin binaries are available with each release; they support repository administration only, not the Windows client/service runtime. See [Repository administration](docs/repo-admin-tools.md).
