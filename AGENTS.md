# AGENTS.md

Guidance for coding agents working in this repository.

## Scope

- Applies to the entire repository unless a deeper `AGENTS.md` overrides it.

## Project Context

- Project: `gorilla` (Windows-focused application/package management tool in Go).
- Main entrypoint: `./cmd/gorilla`.
- Gorilla's deployed/runtime target is Windows, so Windows behavior is first-class.
- CI runs in GitHub Actions and primarily targets Windows (`windows-latest`) to match deployment expectations.
- Development often happens on macOS: keep macOS build/test/dev workflows working where practical, but do not add major complexity solely to preserve parity.
- Where appropriate, macOS/non-Windows stub or no-op behavior is acceptable if it keeps development workflows usable.
- Release assets are published as `gorilla-<version>.msix` (official Windows installer) and `gorilla-<version>.exe` (standalone custom-deployment binary). The MSIX contains the WinUI app plus an internal `gorilla.exe` and registers the automatic `LocalSystem` Gorilla service.
- The packaged product requires Windows 10 version 2004 / build 19041 or newer. Mutable service configuration belongs at `%ProgramData%\gorilla\config.yaml`, outside the MSIX package.

## Preferred Workflow

1. Read relevant package(s) before editing.
2. Make minimal, focused changes.
3. Use the smallest leaf validation target that gives useful feedback while iterating.
4. Before pushing, run the canonical validation level appropriate for the change. `make verify` is the normal portable baseline.
5. Keep changes ready for PR review (clear commits, no unrelated edits).
6. For each task, create and use a new branch named `agent/<task-slug>` (do not work on `main`).
7. Before any commit, verify the current branch was created by this agent for this task; if not, stop and create a new `agent/<task-slug>` branch.
8. Do not commit to or push a branch you did not create unless explicitly asked to.

## Validation Commands

The Makefile is the validation contract for both local development and CI. Prefer these commands over reproducing their underlying `go`, `dotnet`, or integration-script invocations manually.

### Canonical validation levels

- `make verify`
  - Portable validation expected before pushing.
  - Includes Go formatting, Windows-targeted `go vet`, pinned `staticcheck`, race-enabled Go tests, portable .NET analyzer/build checks, and UI Client/Core tests.
- `make verify-windows`
  - Windows only.
  - Adds the real WinUI build and source-built Windows installer/integration validation.
- `make verify-e2e`
  - Windows only.
  - Proves source-built Gorilla service + unpackaged/source-built UI + deterministic fixtures + the critical FlaUI workflows.
- `make verify-release GORILLA_RELEASE_EXE=<path> GORILLA_RELEASE_MSIX=<path>`
  - Windows only and intended for a disposable machine.
  - Runs all lower validation levels, released-binary installer fixture coverage, and installed-product validation against the supplied produced artifacts.
  - Proves the produced MSIX contains the matching `gorilla.exe`, installs/registers the automatic `LocalSystem` service, communicates over the real named pipe, runs the same critical FlaUI workflows through the installed packaged UI, and removes package/service registration on uninstall.

### Reusable leaf targets

Use these for fast focused iteration or CI jobs that own one validation layer:

- `make go-format`
- `make go-vet`
- `make go-staticcheck`
- `make go-test`
- `make ui-restore`
- `make ui-lint`
- `make ui-test`
- `make ui-windows-build`
- `make windows-integration`
- `make ui-e2e` (build and run source-built FlaUI E2E)
- `make ui-e2e-test` (run FlaUI against existing source builds)
- `make release-integration GORILLA_RELEASE_EXE=<path>`
- `make installed-product-integration GORILLA_RELEASE_MSIX=<path> GORILLA_RELEASE_EXE=<path>`

`make lint` and `make test` remain supported compatibility aliases for Go linting and Go tests respectively. `make lint` includes `staticcheck`.

### Which level to use

- Go-only logic: iterate with the relevant Go leaf target(s); run `make verify` before pushing.
- Portable UI Core/Client changes: iterate with `make ui-lint` / `make ui-test`; run `make verify` before pushing.
- Windows service, installer, named-pipe, or Windows-specific behavior: require `make verify-windows` in Windows CI.
- WinUI application behavior: require `make verify-e2e` in Windows CI in addition to portable validation.
- Release/package interoperability changes: use and expand `make verify-release GORILLA_RELEASE_EXE=<produced exe> GORILLA_RELEASE_MSIX=<produced msix>` rather than inventing a separate release-only framework.
- Keep source-built E2E and installed-release validation distinct: source E2E proves source service/UI behavior quickly; installed-product validation proves produced package/service/UI interoperability.

## Build & Test Commands

Other useful make targets:

- `make build`
- `make clean`
- `make bootstrap`
- `make bootstrap-run`
- `make msi` (legacy Windows + WiX package build; not an official release artifact)

Run `make help` for the current command summary.

## Code Style

- Use idiomatic Go and keep code gofmt-clean.
- Prefer small, explicit functions over broad refactors.
- Preserve existing package boundaries (`cmd/`, `pkg/`, `integration/`, `utils/`, `wix/`).
- Keep Gorilla UI code and related docs under `gorilla-ui/` unless there is a clear reason to place files elsewhere.
- Do not add new dependencies unless necessary, and explicitly call out/review any dependency additions in the PR.

## Windows & Integration Notes

- Canonical Windows validation requires PowerShell 7 (`pwsh`).
- On GitHub-hosted Windows runners, keep write-heavy dependency/build caches on the `RUNNER_TEMP` volume (`D:` today) rather than the slower system `C:` drive. The workflows intentionally place Go's `GOMODCACHE`/`GOCACHE` and NuGet's `NUGET_PACKAGES` there.
- Be careful with path handling, newlines, and shell behavior differences.
- Changes that affect service behavior should include/adjust tests in `pkg/service` and `cmd/gorilla` when appropriate.
- When changing Windows named-pipe/service code paths, add or update Windows-only tests (`//go:build windows`) and validate through the Windows validation layer.
- Keep diagnostics pragmatic: prefer debug-level logging or explicit debug toggles over always-on high-volume tracing.
- Manual/integration helpers live under:
  - `integration/windows/`
  - `utils/manual-test/`
- Keep CI-specific setup in workflow YAML only when necessary; the actual build/test/integration behavior should be invoked through repo-owned validation commands or scripts.
- Installed-product validation deliberately refuses to run when an existing Gorilla MSIX, Gorilla service, or `%ProgramData%\gorilla\config.yaml` is present. Use a disposable Windows runner/VM rather than risking a real installation.
- Reuse `prepare-release-integration.ps1` fixture semantics and the existing FlaUI critical workflows for installed-product tests so source and release validation do not drift.

## Config & Examples

- If behavior/config changes, update examples and tests together:
  - `examples/example_config.yaml`
  - `examples/example_catalog.yaml`
  - `examples/example_manifest.yaml`
  - `examples/example_package-info.yaml`
- For UI protocol/data shape changes, keep the JSON contract aligned with the YAML model represented in:
  - `examples/example_manifest.yaml`
  - `examples/example_package-info.yaml`

## UI & Protocol Notes

- Local tooling prerequisites for Gorilla UI work:
  - macOS: `dotnet-sdk@8`
  - Windows: Visual Studio 2022 with WinUI/Windows App SDK tooling and .NET 8 SDK
- UI dependency/ownership boundary:
  - `Gorilla.UI.App` owns XAML, WinUI lifecycle/adapters, Windows runtime paths, and composition.
  - `Gorilla.UI.Core` owns platform-neutral presentation and workflow behavior and must remain plain `net8.0` without WinUI dependencies.
  - `Gorilla.UI.Client` owns named-pipe protocol/client contracts, transport, serialization/validation, and client diagnostics.
  - Runtime dependency direction is `Gorilla.UI.App -> Gorilla.UI.Core -> Gorilla.UI.Client`; App may also reference Client directly at the composition boundary.
- Do not add App-local copies of Core presentation classes or regenerate the committed `Gorilla.UI.App` directory from starter templates.
- Keep `cmd/gorilla` service-message commands updated in lockstep with Gorilla UI protocol changes for testing/debugging.
- `ListOptionalInstalls` should return JSON-safe subset DTOs, not full internal item objects.
- Treat explicit WinUI `AutomationProperties.AutomationId` values used by tests as a compatibility surface. Do not rename or remove them casually.
- Use stable automation IDs for interactive or diagnostically important controls; do not make test identity depend on visible text or layout unless the visible text itself is the behavior under test.
- Repeated templated controls may reuse an automation ID when tests scope the lookup to the relevant item/container. Do not encode changing item data into automation IDs unless global uniqueness is genuinely required.
- Automation IDs do not replace accessibility semantics. Preserve meaningful accessible names, roles, keyboard/focus behavior, and state/value exposure when changing UI controls.

## Diagnostics

- For diagnostics policy, behavior, and implementation guidance, follow:
  - `gorilla-ui/ARCHITECTURE.md` (Diagnostics Decision Record)
  - `gorilla-ui/README.md` (Diagnostics strategy)

## PR Expectations

- Keep PRs focused and explain user-visible behavior changes clearly.
- Call out risks and platform impact (especially Windows).
- Do not include unrelated formatting-only churn.
- After pushing a new branch, open a PR targeting `main`.
- Keep PR title and summary brief but descriptive, and ensure the summary covers all branch changes relative to `main`.
- Only push/open PR for the `agent/<task-slug>` branch created for the current task.

## Safety Rules

- Avoid destructive git commands unless explicitly requested.
- Never remove or overwrite user-authored changes outside the task scope.
- If you encounter unexpected repo state, stop and ask before proceeding.
