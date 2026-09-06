# Mutation testing

Gorilla uses mutation testing selectively to assess whether focused unit tests detect meaningful changes in high-value pure logic. Mutation testing is a diagnostic quality tool, not part of the required `verify*` validation contract.

## Commands

Run the selected suites from the repository root:

```text
make mutation-go
make mutation-ui
make mutation
```

- `make mutation-go` runs pinned Gremlins against `pkg/manifest`.
- `make mutation-ui` restores the repository-local Stryker.NET tool and runs it against selected `Gorilla.UI.Core` behavior.
- `make mutation` composes both suites.

The Go tool version is pinned by `GREMLINS_VERSION` in the Makefile. Stryker.NET is pinned in `.config/dotnet-tools.json` and configured by `gorilla-ui/tests/Gorilla.UI.Core.Tests/stryker-config.json`.

## Manual GitHub Actions runs

After `.github/workflows/mutation-tests.yml` is present on the default branch, use **Actions > Mutation Tests > Run workflow** to run mutation testing without making it part of normal PR validation.

The workflow accepts a `suite` choice:

- `all` runs both mutation suites.
- `go` runs only Gremlins.
- `ui` runs only Stryker.NET.

The workflow can be dispatched against a selected branch or tag, so after this workflow is merged you can still choose a later PR branch when you want to investigate its mutation behavior. Each mutation job has a 60-minute timeout. Gremlins console output and Stryker reports are uploaded as workflow artifacts and retained for 14 days.

The workflow has no `push`, `pull_request`, or scheduled trigger. It is intentionally manual-only and is not a required status check.

## Scope

Keep mutation scope deliberately narrow. Prefer deterministic, branch-heavy logic whose behavior should already be described by ordinary unit tests.

The initial Go scope is `pkg/manifest`. The initial .NET scope covers:

- `OptionalInstallsStartupLoader`
- `OperationTracker`
- `HomeViewModel`

Do not expand mutation testing simply to increase the number of mutants. Avoid Windows service wrappers, installer/process adapters, WinUI code-behind, generated files, and FlaUI tests unless a concrete future need justifies the cost and signal quality.

## Interpreting survivors

A surviving mutant is a prompt to inspect the behavior, not an automatic failure requiring production changes.

For each useful survivor:

1. Determine whether the mutation represents meaningful observable behavior.
2. If an existing test should distinguish it, strengthen the assertion or add the missing behavioral case.
3. If the mutant is equivalent or otherwise not behaviorally meaningful, leave it alone or exclude it narrowly only when the exclusion improves signal.
4. If investigation exposes a real product defect or ambiguous product contract, fix that behavior separately rather than changing production code merely to improve mutation score.

Do not target a repository-wide mutation percentage and do not make mutation testing an every-PR gate unless measured runtime and signal quality justify that policy later.

## Reports and cleanup

Stryker writes local reports under `StrykerOutput/`; these outputs are ignored by Git and removed by `make clean`. Local Gremlins runs report results directly to the console. Manual GitHub Actions runs preserve both suites' output as workflow artifacts for investigation.
