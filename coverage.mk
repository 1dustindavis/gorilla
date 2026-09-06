# Coverage reporting targets are kept separate from the canonical validation
# targets so collecting/reporting coverage does not change `make verify`.
# Compose this file with the main Makefile:
#   make -f Makefile -f coverage.mk coverage

COVERAGE_ROOT ?= $(CURDIR)/build/coverage
GO_COVERAGE_DIR ?= $(COVERAGE_ROOT)/go
UI_COVERAGE_DIR ?= $(COVERAGE_ROOT)/ui
GO_COVERAGE_PROFILE ?= $(GO_COVERAGE_DIR)/coverage.out
GO_COVERAGE_SUMMARY ?= $(GO_COVERAGE_DIR)/summary.txt

.PHONY: coverage coverage-go coverage-ui

coverage-go: gomodcheck
	mkdir -p "$(GO_COVERAGE_DIR)"
	go test -race -covermode=atomic -coverprofile="$(GO_COVERAGE_PROFILE)" ./...
	go tool cover -func="$(GO_COVERAGE_PROFILE)" | tee "$(GO_COVERAGE_SUMMARY)"

coverage-ui: ui-lint
	rm -rf "$(UI_COVERAGE_DIR)"
	mkdir -p "$(UI_COVERAGE_DIR)/client" "$(UI_COVERAGE_DIR)/core"
	dotnet test gorilla-ui/tests/Gorilla.UI.Client.Tests/Gorilla.UI.Client.Tests.csproj --no-build --no-restore --collect:"XPlat Code Coverage" --results-directory "$(UI_COVERAGE_DIR)/client" -- DataCollectionRunSettings.DataCollectors.DataCollector.Configuration.Format=cobertura
	dotnet test gorilla-ui/tests/Gorilla.UI.Core.Tests/Gorilla.UI.Core.Tests.csproj --no-build --no-restore --collect:"XPlat Code Coverage" --results-directory "$(UI_COVERAGE_DIR)/core" -- DataCollectionRunSettings.DataCollectors.DataCollector.Configuration.Format=cobertura

coverage: coverage-go coverage-ui
