# Gorilla UI

This folder contains the WinUI App Catalog and its portable Core/Client layers. Current product architecture is documented in [ARCHITECTURE.md](ARCHITECTURE.md); the staged implementation history is recorded in [issue #208](https://github.com/1dustindavis/gorilla/issues/208).

## Projects and ownership

- `gorilla-ui/src/Gorilla.UI.App/` — WinUI 3 XAML, navigation/lifecycle, Windows runtime composition, accessibility/UIA wiring.
- `gorilla-ui/src/Gorilla.UI.Core/` — platform-neutral presentation/workflow behavior, cache/freshness coordination, operation tracking/recovery, Retry presentation.
- `gorilla-ui/src/Gorilla.UI.Client/` — active v1 named-pipe contracts, transport, serialization/validation, operation lookup/status streaming, diagnostics.
- `gorilla-ui/tests/Gorilla.UI.Client.Tests/` and `gorilla-ui/tests/Gorilla.UI.Core.Tests/` — portable protocol and presentation/workflow coverage.
- `gorilla-ui/tests/Gorilla.UI.App.WindowsUiTests/` — thin Windows/FlaUI validation for behavior that is uniquely valuable at the rendered UI boundary.
- `gorilla-ui/tools/PipeHarness/` — protocol/debug harness.

Runtime dependency direction is `Gorilla.UI.App -> Gorilla.UI.Core -> Gorilla.UI.Client`. Core remains ordinary `net8.0` and WinUI-independent. The Go service owns detection, action authorization, mutation execution, and retained operation truth.

Tooling requirements:
- portable development: .NET 8 SDK plus the normal Go/repository toolchain;
- Windows UI/package validation: Windows with PowerShell 7, WinUI/Windows App SDK tooling, and .NET 8.

## Canonical validation

Use the repository validation levels rather than ad-hoc test commands:

- `make verify` — portable baseline, including Go and Client/Core tests.
- `make verify-windows` — Windows service/installer/named-pipe integration.
- `make verify-e2e` — source-built WinUI/FlaUI critical workflows.
- `make verify-release GORILLA_RELEASE_EXE=<path> GORILLA_RELEASE_MSIX=<path>` — produced binary/MSIX interoperability, installed `LocalSystem` service, packaged UI/service communication, representative fixtures, relaunch recovery, and uninstall cleanup on a disposable Windows host.

`make ui-lint` and `make ui-test` remain useful focused portable commands. `make ui-e2e-test` reuses existing source Windows builds when appropriate.

The Windows UI harness retains screenshots, automation trees, TRX output, client/service logs, and process/service metadata where appropriate. Source UI tests and installed-product tests intentionally reuse the same behavioral infrastructure rather than maintaining parallel UI frameworks.

Stage 7 presentation coverage is deliberately focused: representative Catalog and Details/failure surfaces are validated under baseline/light, 150% scaling, dark app mode, and one high-contrast configuration. This is not an exhaustive DPI/theme/resize matrix.

The installed-product harness verifies that the MSIX installs the Gorilla service as `LocalSystem` and that the packaged App Catalog communicates with it. GitHub-hosted Windows runners launch their interactive session elevated, so an explicit non-elevated/medium-integrity App Catalog process assertion is a manual check in a normal Windows desktop session rather than a hosted-CI guarantee.

## Accessibility and UI Automation

The stable keyboard/accessibility/UIA contract is maintained in [docs/accessibility-automation.md](docs/accessibility-automation.md). In particular:

- preserve existing `AutomationProperties.AutomationId` values unless a separately reviewed semantic change requires otherwise;
- prefer automation identity/control type/state over visible text, coordinates, or template layout;
- scope repeated child IDs to their logical item/operation container;
- Catalog item identity is the canonical protocol `ItemName`;
- Activity/recovery identity is the immutable `OperationId`;
- keyboard tests use actual keyboard input for keyboard-specific behavior;
- virtualized collections are located/restored by logical identity rather than realized-row counts.

Current shell IDs include `CatalogFreshnessStatus`, `CatalogRefreshButton`, `CatalogDegradedWarning`, `InfrastructureWarning`, and `NavigationFrame`.

Current Catalog IDs include `HomeHeading`, `ActivityNavigationButton`, `CatalogSearchBox`, `CatalogItems`, `ItemName`-identified containers, and repeated child roles such as `CatalogCard`, `CatalogObservation`, `CatalogOperationStatus`, `PrimaryActionButton`, and `SecondaryActionButton`.

Current Details IDs include `DetailsBackButton`, `AppDetailsRoot`, `DetailsObservation`, `DetailsActiveOperation`, `DetailsLatestResult`, `DetailsPrimaryAction`, and `DetailsSecondaryAction`. Operation-specific failure/Retry/technical-detail controls are rooted in `OperationId`.

Current Activity IDs include `ActivityPageRoot`, `ActivityBackButton`, `ActivityHeading`, `ActivityItems`, and `ActivityOperation-{OperationId}` rows with operation-ID-rooted child controls.

## Diagnostics

UI client diagnostics are disabled by default. Enable them with `GORILLA_UI_DEBUG=1` (or `GORILLA_DEBUG=1`) before launch. Service named-pipe trace logging remains debug-only (`debug: true` or `--debug`), while normal Gorilla process logging remains available through `gorilla.log`.

Log locations:
- UI client: `%LOCALAPPDATA%\gorilla\ui-client.log`;
- Gorilla service/CLI: `<app_data_path>/gorilla.log` (normally `%ProgramData%\gorilla\gorilla.log`).

Logs use bounded rotation with one backup; logging/rotation failures are best-effort and must not fail product operations.

## Related documentation

- [ARCHITECTURE.md](ARCHITECTURE.md) — current implementation architecture.
- [docs/app-catalog-contract.md](docs/app-catalog-contract.md) — state/action/result contract.
- [docs/app-catalog-recovery.md](docs/app-catalog-recovery.md) — mutation and recovery semantics.
- [docs/accessibility-automation.md](docs/accessibility-automation.md) — keyboard/accessibility/UIA contract.
- [../docs/app-catalog.md](../docs/app-catalog.md) — user/operator App Catalog documentation.
- [PLAN.md](PLAN.md) — historical pointer only; not an active roadmap.
