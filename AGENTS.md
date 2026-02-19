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
3. Run broad local validation with `make test` (tests are fast/lightweight in this repo).
4. Keep changes ready for PR review (clear commits, no unrelated edits).
5. For each task, create and use a new branch named `agent/<task-slug>` (do not work on `main`).
6. Before any commit, verify the current branch was created by this agent for this task; if not, stop and create a new `agent/<task-slug>` branch.
7. Do not commit to or push a branch you did not create unless explicitly asked to.

## Build & Test Commands

- Helpful make targets:
  - `make build`
  - `make test`
  - `make ui-lint`
  - `make ui-test`
  - `make clean`
  - `make bootstrap`
  - `make bootstrap-run`

Prefer `make test` as the default local validation step, even for small changes.
When changes include Gorilla UI/.NET code, run `make ui-lint` and `make ui-test`.
When changes span Go service/CLI and UI protocol layers, run `make test`, `make ui-lint`, and `make ui-test`.

## Code Style

- Use idiomatic Go and keep code gofmt-clean.
- Prefer small, explicit functions over broad refactors.
- Preserve existing package boundaries (`cmd/`, `pkg/`, `integration/`, `utils/`, `wix/`).
- Keep Gorilla UI code and related docs under `gorilla-ui/` unless there is a clear reason to place files elsewhere.
- Do not add new dependencies unless necessary, and explicitly call out/review any dependency additions in the PR.

## Windows & Integration Notes

- Be careful with path handling, newlines, and shell behavior differences.
- Changes that affect service behavior should include/adjust tests in `pkg/service` and `cmd/gorilla` when appropriate.
- When changing Windows named-pipe/service code paths, add or update Windows-only tests (`//go:build windows`) and validate on a Windows VM.
- Keep diagnostics pragmatic: prefer debug-level logging or explicit debug toggles over always-on high-volume tracing.
- Manual/integration helpers live under:
  - `integration/windows/`
  - `utils/manual-test/`

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
  - Windows VM: Visual Studio 2022 with WinUI/Windows App SDK tooling and .NET 8 SDK
- Keep `cmd/gorilla` service-message commands updated in lockstep with Gorilla UI protocol changes for testing/debugging.
- `ListOptionalInstalls` should return JSON-safe subset DTOs, not full internal item objects.

## Diagnostics Handoff Notes (TODO 2)

- When working diagnostics strategy tasks, treat `gorilla-ui/PLAN.md` item 2 as the source of truth for scope and acceptance criteria.
- Produce both policy and implementation guidance in the same PR:
  - policy decisions in `gorilla-ui/ARCHITECTURE.md`
  - operator/developer usage details in `gorilla-ui/README.md`
  - service-side logging notes in relevant Go docs/comments when behavior changes
- Keep diagnostics defaults quiet by default:
  - no high-volume logs unless debug/diagnostic toggles are explicitly enabled
  - avoid creating log directories/files when diagnostics are disabled unless required for existing behavior compatibility
- Ensure policy decisions are concrete and testable:
  - explicit log path(s) by environment
  - explicit retention/rotation trigger and limits
  - explicit required correlation fields (`requestId`, `operationId`) for UI/service troubleshooting
- For diagnostics behavior changes, call out Windows impact in PR summary and run `make test`; include `make ui-lint` and `make ui-test` when touching UI/.NET diagnostics code.

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
