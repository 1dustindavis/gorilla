# Coverage reporting

Gorilla collects coverage so we can spot places where a change may have reduced useful test coverage. It is not meant to be a score we optimize across the whole repository.

For now, coverage is informational. We are collecting a baseline before deciding whether any future regression checks would be useful.

## Run it locally

Use the same Makefile interface as the rest of Gorilla's validation:

```sh
make coverage-go
make coverage-ui
make coverage
```

`make coverage-go` runs the race-enabled Go tests and writes:

- `build/coverage/go/coverage.out` — the Go coverage profile.
- `build/coverage/go/summary.txt` — a readable `go tool cover -func` summary.

`make coverage-ui` runs the portable UI tests and writes Cobertura reports under:

- `build/coverage/ui/client/`
- `build/coverage/ui/core/`

Everything goes under `build/`, so the reports are disposable local output.

## In CI

The normal Go and portable UI workflows collect coverage while they run the tests. GitHub Actions shows a short summary and keeps the full reports as 14-day artifacts.

There is no coverage threshold. Tests can still fail the workflow normally, but a lower coverage percentage by itself will not.

## How to use the reports

Coverage is most useful for questions like:

- Did this change make an important code path less tested?
- Is a pure-logic package or UI Core workflow missing a useful test case?
- Did we add a branch that should have a deterministic unit test?

Don't add tests just to make the percentage go up, and don't change production code just to improve coverage. Windows adapters, generated or stub code, and integration-heavy code may naturally have very different coverage from pure application logic.

These reports give us a baseline. If we add a coverage ratchet later, it should be based on what the data shows and should probably focus on important packages or projects instead of one repository-wide percentage.
