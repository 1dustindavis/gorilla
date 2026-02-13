# Stolen from https://github.com/airbnb/gosal/blob/master/Makefile
all: build

.PHONY: build bootstrap bootstrap-run manual-test-server test lint clean help

ifndef ($(GOPATH))
	GOPATH = $(HOME)/go
endif

PATH := $(GOPATH)/bin:$(PATH)
VERSION = $(shell git describe --tags --always --dirty)
VERSION_NO_PREFIX = $(patsubst v%,%,$(VERSION))
MSI_VERSION = $(word 1,$(subst +, ,$(word 1,$(subst -, ,$(VERSION_NO_PREFIX)))))
BRANCH = $(shell git rev-parse --abbrev-ref HEAD)
REVISION = $(shell git rev-parse HEAD)
REVSHORT = $(shell git rev-parse --short HEAD)
APP_NAME = gorilla
MANUAL_TEST_DIR = build/manual-test
MANUAL_TEST_SERVER_ROOT = ${MANUAL_TEST_DIR}/server-root
MANUAL_TEST_VM_DIR = ${MANUAL_TEST_DIR}/vm
MANUAL_TEST_BASE_URL ?=
GO111MODULE = on

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

	make deps          - Install dependent programs and libraries
	make clean         - Delete all build artifacts

	make build         - Build the code
	make msi           - Build Windows MSI (requires WiX on Windows)
	make bootstrap     - Build manual-test assets/server and generate VM scripts
	make bootstrap-run - Build manual-test assets/server and run local test server

	make test          - Run the Go tests
	make lint          - Run the Go linters

endef

help:
	$(info $(HELP_TEXT))

gomodcheck:
	@go help mod > /dev/null || (@echo gorilla requires Go version 1.11 or higher && exit 1)

clean:
	rm -rf build/

.pre-build: gomodcheck
	mkdir -p build/

build: .pre-build
	GOOS=windows GOARCH=amd64 go build -o build/${APP_NAME}.exe -ldflags ${BUILD_VERSION} ./cmd/gorilla

msi: build
ifeq ($(OS), Windows_NT)
	powershell -Command "$$env:PRODUCT_VERSION='${MSI_VERSION}'; cd wix; ./make-msi.bat"
else
	@echo "msi target requires Windows and WiX"
	@exit 1
endif

manual-test-server: .pre-build
	cd utils/manual-test/server && go build -o ../../../build/manual-test-server .

bootstrap: build manual-test-server
	mkdir -p ${MANUAL_TEST_SERVER_ROOT}/manifests
	mkdir -p ${MANUAL_TEST_SERVER_ROOT}/catalogs
	mkdir -p ${MANUAL_TEST_SERVER_ROOT}/packages
	mkdir -p ${MANUAL_TEST_VM_DIR}
	cp build/${APP_NAME}.exe ${MANUAL_TEST_SERVER_ROOT}/gorilla.exe
	cp examples/example_manifest.yaml ${MANUAL_TEST_SERVER_ROOT}/manifests/example_manifest.yaml
	cp examples/example_catalog.yaml ${MANUAL_TEST_SERVER_ROOT}/catalogs/example_catalog.yaml
	cp utils/manual-test/bootstrap-vm.ps1 ${MANUAL_TEST_VM_DIR}/bootstrap-vm.ps1
	cp utils/manual-test/bootstrap-vm-full.ps1 ${MANUAL_TEST_VM_DIR}/bootstrap-vm-full.ps1
	cp utils/manual-test/templates/run-gorilla-check.bat ${MANUAL_TEST_VM_DIR}/run-gorilla-check.bat
	cp utils/manual-test/run-release-integration.bat ${MANUAL_TEST_VM_DIR}/run-release-integration.bat
	@BASE_URL="${MANUAL_TEST_BASE_URL}"; \
	if [ -z "$$BASE_URL" ]; then \
	  if [ "$(CURRENT_PLATFORM)" = "darwin" ]; then \
	    IFACE=$$(route -n get default 2>/dev/null | awk '/interface:/{print $$2}' | head -n1); \
	    IP_ADDR=$$(ipconfig getifaddr "$$IFACE" 2>/dev/null || true); \
	  elif [ "$(CURRENT_PLATFORM)" = "linux" ]; then \
	    IP_ADDR=$$(hostname -I 2>/dev/null | awk '{print $$1}'); \
	  else \
	    IP_ADDR=""; \
	  fi; \
	  if [ -z "$$IP_ADDR" ]; then IP_ADDR="localhost"; fi; \
	  BASE_URL="http://$$IP_ADDR:8080/"; \
	fi; \
	sed 's#@DEFAULT_BASE_URL@#'"$$BASE_URL"'#g' utils/manual-test/templates/bootstrap-vm.bat > ${MANUAL_TEST_VM_DIR}/bootstrap-vm.bat; \
	sed 's#@DEFAULT_BASE_URL@#'"$$BASE_URL"'#g' utils/manual-test/templates/bootstrap-vm-full.bat > ${MANUAL_TEST_VM_DIR}/bootstrap-vm-full.bat; \
	echo "$$BASE_URL" > ${MANUAL_TEST_VM_DIR}/base-url.txt; \
	echo "Using manual-test base URL: $$BASE_URL"
	@echo "Prepared manual-test assets in ${MANUAL_TEST_SERVER_ROOT}"
	@echo "Run: ./build/manual-test-server -root ${MANUAL_TEST_SERVER_ROOT} -addr :8080"
	@echo "Generated VM scripts in ${MANUAL_TEST_VM_DIR}"

bootstrap-run: bootstrap
	./build/manual-test-server -root ${MANUAL_TEST_SERVER_ROOT} -addr :8080

test: gomodcheck
	go test -cover -race ./...

lint:
	@if gofmt -l -s ./cmd/ ./pkg/ | grep .go; then \
	  echo "^- Repo contains improperly formatted go files; run gofmt -w -s *.go" && exit 1; \
	  else echo "All .go files formatted correctly"; fi
	GOOS=windows GOARCH=amd64 go vet ./...
	golint -set_exit_status `go list ./... | grep -v /vendor/`
