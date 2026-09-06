# Mutation testing

Mutation testing helps us find tests that run code without really proving its behavior. It is optional and is not part of the normal `verify*` checks.

## Run it

From the repo root:

```text
make mutation-go
make mutation-ui
make mutation
```

- `make mutation-go` runs Gremlins against `pkg/manifest`.
- `make mutation-ui` runs Stryker.NET against selected `Gorilla.UI.Core` logic.
- `make mutation` runs both.

You can also run the manual **Mutation Tests** workflow in GitHub Actions and choose `all`, `go`, or `ui`. The workflow is manual-only and does not run on pushes or pull requests.

## What we mutate

Keep the scope small and useful.

Go:

- `pkg/manifest`

.NET:

- `OptionalInstallsStartupLoader`
- `OperationTracker`
- `HomeViewModel`

Avoid mutating Windows service wrappers, installer/process adapters, WinUI code-behind, generated files, and FlaUI tests unless there is a good reason to expand the scope.

## What to do with survivors

A surviving mutant means a test did not catch a code change. Check whether that change matters.

- If it represents real behavior, improve the test.
- If it is equivalent or meaningless, ignore it or exclude it narrowly if that makes the results easier to use.
- If it exposes a real product bug, fix the bug separately instead of changing production code just to improve the mutation score.

We are not enforcing a mutation score or running mutation tests on every PR. We can revisit that after we have enough runtime and signal data.

## Reports

The GitHub Actions workflow keeps mutation artifacts for 14 days.

- Gremlins output is saved from the Go job.
- Stryker reports are saved from `StrykerOutput/`.

Local Stryker output is ignored by Git and removed by `make clean`.
