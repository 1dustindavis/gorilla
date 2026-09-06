# Coverage reporting

Gorilla uses coverage as a regression and review signal, not as a repository-wide score target. Stage 10 of the automated-testing work deliberately starts by measuring and publishing the current baseline without introducing a blocking coverage threshold.

## Run locally

Coverage uses the same Makefile validation interface as the rest of the repository:

```sh
make coverage-go
make coverage-ui
make coverage
```

`coverage-go` runs the race-enabled Go suite and writes:

- `build/coverage/go/coverage.out` — Go's machine-readable coverage profile.
- `build/coverage/go/summary.txt` — `go tool cover -func` output for quick review.

`coverage-ui` builds the portable UI test projects and writes Cobertura XML reports beneath:

- `build/coverage/ui/client/`
- `build/coverage/ui/core/`

The generated files are under `build/`, which is already treated as disposable repository output.

## CI behavior

The existing Go and portable UI workflows collect coverage while running their normal test suites. They publish concise coverage information to the GitHub Actions step summary and upload the machine-readable reports as 14-day workflow artifacts.

Coverage reporting does **not** add a separate pass/fail percentage. A test failure still fails validation normally; a lower coverage percentage by itself does not.

## Interpreting coverage

Use coverage to answer questions such as:

- Did a change accidentally make important behavior less tested?
- Is a pure-logic package or UI Core workflow missing meaningful cases?
- Does a new branch represent behavior that deserves a deterministic unit test?

Do not add tests solely to increase a number, and do not change production behavior merely to improve coverage. Windows adapters, generated/stub code, and integration-oriented boundaries can have different useful coverage characteristics than pure application logic.

The current reports establish the baseline needed for a future ratchet. Any later enforcement should be introduced separately, after the baseline is reviewed, and should prefer important package/project-level regression protection over an arbitrary repository-wide target.
