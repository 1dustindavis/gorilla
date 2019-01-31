# Gorilla
[![Go Report Card](https://goreportcard.com/badge/github.com/1dustindavis/gorilla)](https://goreportcard.com/report/github.com/1dustindavis/gorilla) [![Build status](https://ci.appveyor.com/api/projects/status/hvug2p5wsvlor2v0/branch/master?svg=true)](https://ci.appveyor.com/project/DustinDavis/gorilla/branch/master)

Munki-like Application Management for Windows

Gorilla is intended to provide application management on Windows using [Munki](https://github.com/munki/munki) as inspiration.
Gorilla supports `.msi`, `.ps1`, `.exe`, or `.nupkg` [(via chocolatey)](https://github.com/chocolatey/choco).

## Getting Started
Information related to installing and configuring Gorilla can be found on the [Wiki](https://github.com/1dustindavis/gorilla/wiki)

## Building

If you just want the latest version, get it here: https://github.com/1dustindavis/gorilla/releases

Building from source requires the golang tools: https://golang.org/doc/install

#### macOS
After cloning this repo, run `make deps` and then `make build`. A new binary will be created in `build/`

#### Windows
1. Clone this repo
2. Install dep: `go get -u github.com/golang/dep/...`
4. Install dependencies: `dep ensure -vendor-only -v`
5. Build gorilla: `go build -i ./cmd/gorilla`

## Contributing
Pull Requests are always welcome. Before submitting, check if the linter or tests fail:
```
go fmt ./...
go test ./...
```
