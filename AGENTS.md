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
  - Includes Go formatting, Windows-targeted `go vet`, pinned `staticcheck`, race-enabled Go tests, portable .NET analyzer/build checks, and UI client tests.
- `make verify-windows`
  - Windows only.
  - Adds the real WinUI build and source-built Windows installer/integration validation.
- `make verify-e2e`
  - Windows only.
  - Adds the current FlaUI full-application UI suite.
- `make verify-release GORILLA_RELEASE_EXE=<path>`
  - Windows only.
  - Runs the lower validation levels and the current released-binary integration against the supplied produced `gorilla.exe` artifact.
  - The release-level contract is intentionally stable; installed MSI + service + MSIX UI coverage will expand behind this command as that infrastructure is added.

### Reusable leaf targets

Use these for fast focused iteration or CI jobs that own one validation layer:

- `make go-format`
- `make go-vet`
- `make go-staticcheck`
- `make go-test`
- `make ui-lint`
- `make ui-test`
- `make ui-windows-build`
- `make windows-integration`
- `make ui-e2e` (build and run the FlaUI suite)
- `make ui-e2e-test` (run FlaUI against an existing Windows UI build)
- `make release-integration GORILLA_RELEASE_EXE=<path>`

`make lint` and `make test` remain supported compatibility aliases for Go linting and Go tests respectively. `make lint` includes `staticcheck`.

### Which level to use

- Go-only logic: iterate with the relevant Go leaf target(s); run `make verify` before pushing.
- Portable UI client/protocol changes: iterate with `make ui-lint` / `make ui-test`; run `make verify` before pushing.
- Windows service, installer, named-pipe, or Windows-specific behavior: require `make verify-windows` in Windows CI.
- WinUI application behavior: require `make verify-e2e` in Windows CI in addition to portable validation.
- Release/package interoperability changes: use `make verify-release GORILLA_RELEASE_EXE=<produced artifact>` where the released-binary validation applies; expand this same level rather than inventing a separate release-only validation path.

## Build & Test Commands

Other useful make targets:

- `make build`
- `make clean`
- `make bootstrap`
- `make bootstrap-run`
- `make msi` (Windows + WiX)

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
- Keep `cmd/gorilla` service-message commands updated in lockstep with Gorilla UI protocol changes for testing/debugging.
- `ListOptionalInstalls` should return JSON-safe subset DTOs, not full internal item objects.

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
