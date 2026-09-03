all: build

.PHONY: build bootstrap bootstrap-run manual-test-server clean help \
	go-format go-vet go-staticcheck go-test lint test \
	ui-restore ui-lint ui-test ui-windows-build ui-e2e ui-e2e-test \
	windows-integration release-integration installed-product-integration \
	verify verify-windows verify-e2e verify-release

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
STATICCHECK_VERSION ?= v0.7.0
MANUAL_TEST_DIR = build/manual-test
MANUAL_TEST_SERVER_ROOT = ${MANUAL_TEST_DIR}/server-root
MANUAL_TEST_VM_DIR = ${MANUAL_TEST_DIR}/vm
MANUAL_TEST_BASE_URL ?=
WINDOWS_INTEGRATION_WORK_ROOT ?= $(CURDIR)/build/windows-integration
UI_E2E_WORK_ROOT ?= $(WINDOWS_INTEGRATION_WORK_ROOT)
RELEASE_INTEGRATION_WORK_ROOT ?= $(CURDIR)/build/release-integration
INSTALLED_PRODUCT_WORK_ROOT ?= $(CURDIR)/build/installed-product-integration
RELEASE_USE_PREBUILT_FIXTURES ?= 0
GORILLA_RELEASE_EXE ?=
GORILLA_RELEASE_MSIX ?=
GO111MODULE = on

ifneq ($(OS), Windows_NT)
	CURRENT_PLATFORM = linux
	ifeq ($(shell uname), Darwin)
		SHELL := /bin/bash
		CURRENT_PLATFORM = darwin
	endif
	GOVERSION = $(shell go version | awk '{print $$3}')
	NOW	= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
else
	CURRENT_PLATFORM = windows
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

	make deps           - Install dependent programs and libraries
	make clean          - Delete all build artifacts

	make build          - Build the code
	make msi            - Build legacy Windows MSI (not an official release artifact)
	make bootstrap      - Build manual-test assets/server and generate VM scripts
	make bootstrap-run  - Build manual-test assets/server and run local test server

	make verify         - Run all portable validation expected before pushing
	make verify-windows - Add Windows build and source integration validation
	make verify-e2e     - Add source-built service/app integration and critical FlaUI workflows
	make verify-release GORILLA_RELEASE_EXE=... GORILLA_RELEASE_MSIX=... - Validate produced release artifacts as an installed product plus lower layers

	make go-format       - Check Go formatting
	make go-vet          - Run Go vet for the Windows deployment target
	make go-staticcheck  - Run pinned staticcheck
	make go-test         - Run Go tests with coverage and race detection
	make ui-restore      - Restore portable Gorilla UI .NET dependencies
	make ui-lint         - Run Gorilla UI portable build/analyzer validation
	make ui-test         - Run the Gorilla UI portable .NET tests
	make ui-windows-build - Build the real WinUI app and Windows UI test project
	make windows-integration - Run source-built Windows installer integration
	make ui-e2e          - Build source service/UI and run critical FlaUI E2E workflows
	make ui-e2e-test     - Run critical FlaUI E2E against existing source builds
	make release-integration GORILLA_RELEASE_EXE=... - Test a supplied release binary against installer fixtures
	make installed-product-integration GORILLA_RELEASE_MSIX=... - Install the produced MSIX and validate service/UI/package interoperability

	make lint           - Compatibility alias for Go format/vet/staticcheck
	make test           - Compatibility alias for Go tests

endef

help:
	$(info $(HELP_TEXT))

gomodcheck:
	@go help mod > /dev/null || (@echo gorilla requires Go version 1.11 or higher && exit 1)

clean:
	rm -rf build/
	rm -rf gorilla-ui/src/Gorilla.UI.Client/bin/
	rm -rf gorilla-ui/src/Gorilla.UI.Client/obj/
	rm -rf gorilla-ui/src/Gorilla.UI.Core/bin/
	rm -rf gorilla-ui/src/Gorilla.UI.Core/obj/
	rm -rf gorilla-ui/tests/Gorilla.UI.Client.Tests/bin/
	rm -rf gorilla-ui/tests/Gorilla.UI.Client.Tests/obj/
	rm -rf gorilla-ui/tests/Gorilla.UI.Client.Tests/TestResults/
	rm -rf gorilla-ui/tests/Gorilla.UI.Core.Tests/bin/
	rm -rf gorilla-ui/tests/Gorilla.UI.Core.Tests/obj/
	rm -rf gorilla-ui/tests/Gorilla.UI.Core.Tests/TestResults/
	rm -rf gorilla-ui/tools/PipeHarness/bin/
	rm -rf gorilla-ui/tools/PipeHarness/obj/
	rm -rf gorilla-ui/src/Gorilla.UI.App/AppPackages/
	rm -rf gorilla-ui/src/Gorilla.UI.App/bin/
	rm -rf gorilla-ui/src/Gorilla.UI.App/obj/

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

# Portable validation leaf targets. Keep these small so CI and developers can
# run exactly the layer they need while the verify targets compose the contract.
go-format:
	@UNFORMATTED="$$(git ls-files '*.go' | xargs gofmt -l -s)"; \
	if [ -n "$$UNFORMATTED" ]; then \
	  echo "Repo contains improperly formatted Go files:"; \
	  echo "$$UNFORMATTED"; \
	  exit 1; \
	else \
	  echo "All Go files formatted correctly"; \
	fi

go-vet: gomodcheck
	GOOS=windows GOARCH=amd64 go vet ./...

go-staticcheck: gomodcheck
	go run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION) ./...

go-test: gomodcheck
	go test -cover -race ./...

ui-restore:
	dotnet restore gorilla-ui/tests/Gorilla.UI.Client.Tests/Gorilla.UI.Client.Tests.csproj
	dotnet restore gorilla-ui/tests/Gorilla.UI.Core.Tests/Gorilla.UI.Core.Tests.csproj
	dotnet restore gorilla-ui/tools/PipeHarness/PipeHarness.csproj

ui-lint: ui-restore
	dotnet build gorilla-ui/tests/Gorilla.UI.Client.Tests/Gorilla.UI.Client.Tests.csproj --no-restore -warnaserror
	dotnet build gorilla-ui/tests/Gorilla.UI.Core.Tests/Gorilla.UI.Core.Tests.csproj --no-restore -warnaserror
	dotnet build gorilla-ui/tools/PipeHarness/PipeHarness.csproj --no-restore -warnaserror

ui-test: ui-lint
	dotnet test gorilla-ui/tests/Gorilla.UI.Client.Tests/Gorilla.UI.Client.Tests.csproj --no-build --no-restore
	dotnet test gorilla-ui/tests/Gorilla.UI.Core.Tests/Gorilla.UI.Core.Tests.csproj --no-build --no-restore

ui-windows-build:
ifeq ($(OS), Windows_NT)
	dotnet build gorilla-ui/src/Gorilla.UI.App/Gorilla.UI.App.csproj -c Release -p:Platform=x64 -p:WindowsPackageType=None -p:WindowsAppSDKSelfContained=true -p:PublishReadyToRun=false -p:PublishTrimmed=false
	dotnet build gorilla-ui/tests/Gorilla.UI.App.WindowsUiTests/Gorilla.UI.App.WindowsUiTests.csproj -c Release
else
	@echo "ui-windows-build requires Windows"
	@exit 1
endif

windows-integration: build
ifeq ($(OS), Windows_NT)
	pwsh -NoProfile -ExecutionPolicy Bypass -File integration/windows/run-source-integration.ps1 -WorkRoot "$(WINDOWS_INTEGRATION_WORK_ROOT)" -GorillaExePath "$(CURDIR)/build/gorilla.exe"
else
	@echo "windows-integration requires Windows"
	@exit 1
endif

ui-e2e: build ui-windows-build
ifeq ($(OS), Windows_NT)
	$(MAKE) ui-e2e-test
else
	@echo "ui-e2e requires Windows"
	@exit 1
endif

ui-e2e-test:
ifeq ($(OS), Windows_NT)
	pwsh -NoProfile -ExecutionPolicy Bypass -File integration/windows/run-ui-e2e.ps1 -WorkRoot "$(UI_E2E_WORK_ROOT)" -GorillaExePath "$(CURDIR)/build/gorilla.exe"
else
	@echo "ui-e2e-test requires Windows"
	@exit 1
endif

release-integration:
ifeq ($(OS), Windows_NT)
ifeq ($(strip $(GORILLA_RELEASE_EXE)),)
	@echo "GORILLA_RELEASE_EXE must point to a produced gorilla.exe release artifact"
	@exit 1
else
ifeq ($(RELEASE_USE_PREBUILT_FIXTURES),1)
	pwsh -NoProfile -ExecutionPolicy Bypass -File integration/windows/run-source-integration.ps1 -WorkRoot "$(RELEASE_INTEGRATION_WORK_ROOT)" -GorillaExePath "$(GORILLA_RELEASE_EXE)" -UsePrebuiltFixtures
else
	pwsh -NoProfile -ExecutionPolicy Bypass -File integration/windows/run-source-integration.ps1 -WorkRoot "$(RELEASE_INTEGRATION_WORK_ROOT)" -GorillaExePath "$(GORILLA_RELEASE_EXE)"
endif
endif
else
	@echo "release-integration requires Windows"
	@exit 1
endif

installed-product-integration:
ifeq ($(OS), Windows_NT)
ifeq ($(strip $(GORILLA_RELEASE_MSIX)),)
	@echo "GORILLA_RELEASE_MSIX must point to the produced Gorilla MSIX release artifact"
	@exit 1
else
	pwsh -NoProfile -ExecutionPolicy Bypass -File integration/windows/run-installed-product-integration.ps1 -WorkRoot "$(INSTALLED_PRODUCT_WORK_ROOT)" -GorillaMsixPath "$(GORILLA_RELEASE_MSIX)" -GorillaReleaseExePath "$(GORILLA_RELEASE_EXE)"
endif
else
	@echo "installed-product-integration requires Windows"
	@exit 1
endif

verify: go-format go-vet go-staticcheck go-test ui-lint ui-test

verify-windows: verify
ifeq ($(OS), Windows_NT)
	$(MAKE) ui-windows-build
	$(MAKE) windows-integration
else
	@echo "verify-windows requires Windows"
	@exit 1
endif

verify-e2e: verify-windows
ifeq ($(OS), Windows_NT)
	$(MAKE) ui-e2e-test
else
	@echo "verify-e2e requires Windows"
	@exit 1
endif

# Release validation deliberately composes the same lower layers and fixtures as
# source E2E, then proves the produced MSIX installs the matching gorilla.exe,
# registers the LocalSystem service, communicates over the real named pipe, and
# drives the same critical product workflows through the installed WinUI app.
verify-release: verify-e2e
ifeq ($(OS), Windows_NT)
	$(MAKE) release-integration
	$(MAKE) installed-product-integration
else
	@echo "verify-release requires Windows"
	@exit 1
endif

# Backwards-compatible entry points used by existing workflows and contributors.
lint: go-format go-vet go-staticcheck

test: go-test
