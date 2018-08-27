# Stolen from https://github.com/airbnb/gosal/blob/master/Makefile
all: build

.PHONY: build

ifndef ($(GOPATH))
	GOPATH = $(HOME)/go
endif

PATH := $(GOPATH)/bin:$(PATH)
VERSION = $(shell git describe --tags --always --dirty)
BRANCH = $(shell git rev-parse --abbrev-ref HEAD)
REVISION = $(shell git rev-parse HEAD)
REVSHORT = $(shell git rev-parse --short HEAD)
APP_NAME = gorilla
PKGDIR_TMP = ${TMPDIR}golang
XGO_INSTALLED = $(shell command -v xgo 2> /dev/null)

ifneq ($(OS), Windows_NT)
	CURRENT_PLATFORM = linux
	# If on macOS, set the shell to bash explicitly
	ifeq ($(shell uname), Darwin)
		SHELL := /bin/bash
		CURRENT_PLATFORM = darwin
	endif

	# To populate version metadata, we use unix tools to get certain data
	GOVERSION = $(shell go version | awk '{print $$3}')
	NOW	= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
else
	CURRENT_PLATFORM = windows

	# To populate version metadata, we use windows tools to get the certain data
	GOVERSION_CMD = "(go version).Split()[2]"
	GOVERSION = $(shell powershell $(GOVERSION_CMD))
	NOW	= $(shell powershell Get-Date -format s)
endif

BUILD_VERSION = "\
	-X github.com/1dustindavis/gorilla/pkg/version.appName=${APP_NAME} \
	-X github.com/1dustindavis/gorilla/pkg/version.version=${VERSION} \
	-X github.com/1dustindavis/gorilla/pkg/version.branch=${BRANCH} \
	-X github.com/1dustindavis/gorilla/pkg/version.buildDate=${NOW} \
	-X github.com/1dustindavis/gorilla/pkg/version.revision=${REVISION} \
	-X github.com/1dustindavis/gorilla/pkg/version.goVersion=${GOVERSION}"

define HELP_TEXT

  Makefile commands

	make deps         - Install dependent programs and libraries
	make clean        - Delete all build artifacts

	make build        - Build the code
	make release      - Build in a docker container using xgo

	make test         - Run the Go tests
	make lint         - Run the Go linters

endef

help:
	$(info $(HELP_TEXT))

deps:
	go get -u github.com/golang/dep/...
	go get -u github.com/golang/lint/golint
	dep ensure -vendor-only -v

clean:
	rm -rf build/
	rm -rf release/
	rm -rf ${PKGDIR_TMP}

.pre-build:
	mkdir -p build/

build: .pre-build
	GOOS=windows GOARCH=amd64 go build -i -o build/${APP_NAME}.exe -pkgdir ${PKGDIR_TMP} -ldflags ${BUILD_VERSION} ./cmd/gorilla

.pre-release: clean
	mkdir -p release/

release: .pre-release
ifndef XGO_INSTALLED
	$(error "xgo is not available, please install xgo")
endif
	xgo --targets=windows/amd64 -dest release/ -ldflags ${BUILD_VERSION} ./cmd/gorilla
	mv release/*.exe release/gorilla.exe

test:
	GOOS=windows GOARCH=amd64 go test -cover -race -v $(shell go list ./... | grep -v /vendor/)

lint:
	@if gofmt -l . | egrep -v ^vendor/ | grep .go; then \
	  echo "^- Repo contains improperly formatted go files; run gofmt -w *.go" && exit 1; \
	  else echo "All .go files formatted correctly"; fi
	GOOS=windows GOARCH=amd64 go vet ./...
	# Bandaid until https://github.com/golang/lint/pull/325 is merged
	golint -set_exit_status `go list ./... | grep -v /vendor/`