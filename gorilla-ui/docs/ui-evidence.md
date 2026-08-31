# Windows UI evidence bundle

`make ui-e2e` produces a diagnostic evidence bundle under the E2E work root. In CI the uploaded artifact is named `windows-ui-evidence` and comes from:

```text
build/windows-integration/ui-evidence/
```

Evidence is isolated by whole-scenario attempt and E2E phase so a retry cannot erase the diagnostics from the failed attempt:

```text
ui-evidence/
  attempt-1/
    healthy/
      screenshots/
        healthy-startup.png
        after-install.png
        after-remove.png
      automation-tree.txt
      ui-client.log
      gorilla.log
      process-info.txt
      service-info.txt
      tests-attempt-1.trx
    service-unavailable/
      screenshots/
        cached-service-unavailable.png
      automation-tree.txt
      ui-client.log
      gorilla.log
      process-info.txt
      service-info.txt
      tests-attempt-1.trx
    harness-failure.txt       # only when the scenario attempt fails
  attempt-2/
    ...                       # present only when a retry runs
```

The exact set is best-effort. A failure that prevents the application or service from starting may naturally make some files unavailable. Diagnostic collection must never replace or hide the original test failure.

## What the evidence means

- Screenshots are review/debug evidence, not pixel-golden assertions. Representative successful checkpoints are intentionally captured from the critical workflows that exist today.
- `automation-tree.txt` records the live UI Automation hierarchy and important properties such as control type, automation ID, accessible name, enabled state, and offscreen state.
- `ui-client.log` is produced with `GORILLA_UI_DEBUG=1`; the E2E harness redirects `GORILLA_UI_LOG_PATH` directly into the current phase directory.
- `gorilla.log` is copied from the isolated test service's configured `app_data_path`; E2E also enables service debug logging for protocol troubleshooting.
- `process-info.txt` records the launched UI process ID and lifecycle/exit metadata when available. Failure-specific text diagnostics also embed this process information.
- `service-info.txt` records the Windows service state and process metadata at the end of the phase when available.
- `tests-attempt-1.trx` is the xUnit/.NET test result for that isolated phase. The inner test runner uses one attempt; retries happen at the outer scenario level and therefore get a new `attempt-N` directory.

When diagnosing a retry, inspect the earliest failed `attempt-N` first rather than only the final successful attempt. Correlate `ui-client.log` and `gorilla.log` by request/operation IDs where present, then use the screenshot and automation tree to confirm what WinUI actually exposed at the failing point.
