# App Catalog state and action contract

Stage 1 of [#208](https://github.com/1dustindavis/gorilla/issues/208).

This document records the decisions proposed for the completed optional-software
UI. `pkg/appcatalog` implements the pure action/result decisions; Client's
`AppCatalog` namespace defines matching payload records. Shared examples in
`pkg/appcatalog/testdata/contract.json` are exercised by Go and .NET tests.

**Implementation boundary:** stage 2 keeps the v1 envelope and operation lifecycle
while replacing placeholder list data with real catalog metadata and shared Go
observations. Its transitional list item includes the v2 observation, policy, and
action objects alongside fields consumed by the current UI. The service enforces
those actions. Stage 3 connects item results to execution and moves the whole live
client/service exchange to the complete v2 operation contract. Stage 4 supplies
durable tracking/recovery. Enable the v2 envelope only
when service, CLI, and UI can use the complete contract together; do not label
v1 data as v2 or infer new state from its placeholder fields.

## Product decisions

- Show only effective `optional_installs` entries in App Catalog. Keep entries
  with invalid metadata visible with an explanation, but do not allow actions.
- Install persists the item in the device's local `managed_installs`, removes a
  local removal selection, and requests convergence to the catalog target.
- Remove clears the local install selection and requests one uninstall. It does
  not create a persistent local `managed_uninstalls` entry. Afterward, the app is
  unselected: reinstalling it outside Gorilla must not cause Gorilla to remove it.
  A failed removal keeps a failure result and permits an explicit retry; it does
  not restore the install selection or schedule recurring removal attempts.
  Administrator-controlled `managed_uninstalls` remain independent policy.
- Optional apps selected for installation update automatically. The first UI has
  no separate Update action. An unselected installed app can be adopted through
  Install; a selected app that is absent can be retried through Install.
- Administrator policy takes precedence over user selection. Conflicting
  administrator instructions are reported rather than resolved by execution order.
- Preferred direction: optional requests share the serialized executor and run
  only the requested item and its necessary dependencies. This is conditional on
  the execution-scope validation below; stage 1 does not establish that narrowing
  the run is safe. Scheduled runs must continue to converge all managed items.
- Managed-update notifications, installer cancellation, and automatic dependency
  garbage collection are outside this scope.

The running stage 2 service still schedules a full managed run per accepted
request. It now enforces these action rules before mutation; stage 3 retains the
documented validation gate before narrowing execution.

## Three independent kinds of state

| Kind | Meaning | Examples |
| --- | --- | --- |
| Observation | What fresh detection established on the device | Absent, Installed, UpdateAvailable, Unknown, DetectionFailed |
| Policy/selection | What administrators require and what the user selected | Required install, required removal, local Install/None |
| Operation | What a particular accepted request is doing or how it ended | Queued, Running, Completed with a result |

A selected app can be absent. An installed app can be unselected. A failed
operation can leave an app present. Never overwrite one kind of state with another.
Connection availability and cache freshness are UI/service-health state, not app
presence or terminal operation outcomes.

### Observation

| State | Evidence required | What it does not establish |
| --- | --- | --- |
| Absent | Successful applicable detection establishes absence | That no app with a similar display name exists |
| Installed | Successful detection establishes presence | A known installed version, latest version, or satisfied target |
| UpdateAvailable | Presence plus the selected detection check's valid version comparison establishes an unmet version requirement | That an installer has started |
| Unknown | No check, not checked yet, unsupported or ambiguous evidence | Absence or an installation failure |
| DetectionFailed | An attempted check failed to execute/read/parse reliably | Absence or an installation failure |

- `targetVersion` comes from top-level catalog `version` and is display metadata
  for the offered package, not the authoritative detection threshold.
  `installedVersion` is observed evidence. Either can be null. Never copy the
  target into the installed version or compare these DTO fields to derive state.
- `checkedAtUtc` is the observation/attempt time, not response construction time.
  It is null before an attempt. `detailCode` explains uncertainty or failed detection.
- `installRequirement` records whether the selected check says installation
  work is `Satisfied`, `NotSatisfied`, or `Unknown`. It does not establish
  physical presence or version. This distinction lets legacy script checks
  authorize installation without being mislabeled as Absent or Installed.
- Installed with an unknown version is valid. Do not label it “up to date.”
- An installed version meeting or exceeding the selected check's requirement is
  not an available update or a reason to downgrade.
- Hash/repair requirements are not version-based update availability.

### Version authority

The selected detection check is authoritative for update state and Install
postcondition verification, using existing `pkg/status.CheckStatus` precedence
(script, file, registry with a version, then AppX) and comparison semantics.
Version requirements come from `check.registry.version`, each applicable
`check.file[].version`, or `check.appx.version`. Top-level catalog `version` does
not override or supplement those requirements; there is no equality invariant.
Missing, invalid, or ambiguous check evidence must not fall back to comparing
against `targetVersion`.

For multiple file checks, retain each file's requirements and existing evaluation
semantics in the shared Go implementation. Do not collapse component versions
into one app-wide threshold. A version-based UpdateAvailable observation needs
reliable presence and unmet version evidence; other unmet requirements, such as
hash checks, do not independently establish UpdateAvailable. Script checks retain
the evidence limits below. C# displays the service's state without comparing
version strings.

For example, catalog version `2.0`, registry requirement `1.5`, and installed
version `1.7` means Installed, not UpdateAvailable. With fresh, unambiguous registry
evidence and no other unmet requirements, Install verification can be Satisfied
once selection is persisted. Conversely, catalog version `1.5`, registry
requirement `2.0`, and installed version `1.7` means UpdateAvailable and an unmet
Install postcondition. `TestCheckStatusRegistryVersionAuthority` locks down both
directions through the existing Go check. Stage 2 must carry these same cases
through the richer observation adapter; stage 3 must cover their postconditions.

### Detection adapters to implement in stage 2

The UI retrieves observations from the Go service. There must be one shared Go
detection implementation for CLI, scheduled runs, and App Catalog. Extend/refactor
`pkg/status` to expose the evidence its existing checks already gather; retain
`CheckStatus` as the action-needed interface backed by that shared implementation.
Do not create separate registry/file/script/AppX checks in C#, the service, or
`pkg/appcatalog`. The new package defines data and pure action/result decisions;
it does not detect installation. Reuse the existing Go catalog resolver as well.
For ambiguous registry substring matches, the rich adapter returns Unknown with
no version. The legacy `CheckStatus` interface retains its historical first-match
action decision so stage 2 does not change CLI or scheduled convergence behavior.

Stage 2 resets the shared registry cache once at the start of each catalog
observation pass and managed run. Registry enumeration remains shared within a
pass, while a later refresh or run cannot reuse the earlier snapshot.

The current boolean answers whether an action is needed, not why: installation
may be needed because an app is absent, outdated, or fails a hash check. It cannot
alone populate all the proposed UI fields. Expose richer evidence in Go and send
it over the pipe, preserving CLI behavior through regression tests.

| Check | Observation rules |
| --- | --- |
| Registry | Establish presence from matching uninstall entries; expose a parseable observed version when available. A registry read failure is DetectionFailed. Ambiguous matches cannot justify a version claim. |
| File | A readable required file establishes evidence; all required files present can establish Installed. All absent can establish Absent. Partial presence is Unknown. Access errors are DetectionFailed. Do not invent one app version from conflicting file versions. |
| AppX/MSIX | Observe provisioned package presence/version, matching Gorilla's machine-level scope. Do not present it as proof of launchability for each user's registration. |
| Legacy script | Existing exit codes describe whether installation work is needed, not necessarily presence or version. Keep physical state Unknown, expose the result as a Satisfied or NotSatisfied install requirement, and allow actions from that requirement. Process-start/read errors are DetectionFailed. |
| Missing/unsupported check | Unknown with an explanation. |

Check selection remains script, file, registry, then AppX. Do not execute a
lower-priority check as supplemental evidence when a script is selected; doing
so would silently change long-standing catalog semantics. A script-only item can
be installed from NotSatisfied and adopted/removed from Satisfied while its
physical state remains Unknown. DetectionFailed and Unknown requirement evidence
remain unavailable. CLI/scheduled legacy behavior is unchanged.

## Action policy

The service owns these decisions. Core displays the supplied allowed/reason pair;
it does not duplicate policy. Evaluate against trusted, current data at request
acceptance and again before execution. Cached allowed actions are not authority.

Resolve `requiredInstall` and `requiredUninstall` from administrator-controlled
manifests, excluding the service-generated selection manifest. Local selection
is separately `None` or `Install`. A Remove operation is not a third selection.
Stage 2 must explicitly migrate legacy removal entries from the service-generated
selection manifest so they do not keep enforcing removal. Do not delete
administrator-authored uninstall policy or synthesize new removal jobs from old
entries during migration.

`managed_updates` alone does not require presence and does not block optional
removal. Preserve exact catalog item identity; do not authorize by display name.

`requiredDependency` means the item is still needed by a required or another
selected app. Compute the dependency closure with cycle/missing-item detection.
Removing an app must not remove its dependencies automatically. Installing an
optional app must reject an unsatisfiable dependency or one forced absent by
administrator policy; do not claim success after skipping a dependency failure.

Evaluate the following common blockers in order:

| Condition | Install / Remove | Reason |
| --- | --- | --- |
| Not optional | Denied / Denied | not_optional |
| Required uninstall conflicts with required install or required dependency | Denied / Denied | policy_conflict |
| Required uninstall | Denied / Denied | managed_uninstall |
| Required install | Denied / Denied | required_install |
| Active request for this item | Denied / Denied | operation_active |
| Invalid local selection | Denied / Denied | invalid_selection |
| Unknown/unrecognized observation | Denied / Denied | state_unknown |
| Detection failed | Denied / Denied | detection_failed |

Then apply action-specific rules:

| Action | Condition, in order | Decision/reason |
| --- | --- | --- |
| Install | No valid install recipe | Denied: install_unavailable |
| Install | Already selected Install and present (Installed/UpdateAvailable) | Denied: already_selected; updates remain automatic |
| Install | Otherwise | Allowed, including adoption and retry of an absent selected app |
| Remove | Needed dependency | Denied: required_dependency |
| Remove | Absent and not selected Install | Denied: already_absent |
| Remove | Absent and selected Install | Allowed: withdraw selection without running an uninstaller |
| Remove | Present without a valid removal recipe | Denied: remove_unavailable |
| Remove | Otherwise | Allowed, including retry while still present |

`allowed=true` uses an empty reason. Denied decisions have stable reason codes
that the UI maps to plain-language explanations. Recipe capability includes
catalog validity and dependency feasibility; the pure function does not load or
validate recipes. Keeping selection and presence separate also permits adoption
without reinstalling an already satisfied app.

Policy changes can invalidate old selection. Administrator policy must suppress
conflicting persisted selections in both optional execution and scheduled
convergence; simply rejecting new clicks would not fix that conflict. Implement
this reconciliation before every stage-2 service-managed run. Stage 2 also
validates the full dependency graph before authorizing a new Install selection;
legacy CLI dependency execution remains unchanged.

## Per-item execution and results

Every operation carries `operationId`, `itemName`, and `action` (Install/Remove).
`phase` is Queued, Running, or Completed. Only Completed has a terminal `result`.
An item has at most one active operation plus an optional last completed operation.
The last result must survive list refresh; successful observation does not erase it.

`progressPercent` is null unless measured, including during installer execution
without progress support. Do not use the old fixed 20/60/100 milestones. A result
is immutable once terminal. Connection loss only changes the client's ability to
observe the operation.

| Execution evidence | Target/selection verification | Terminal outcome |
| --- | --- | --- |
| Error in requested work or necessary dependency | Any | Failed / execution_failed |
| Execution interrupted with no trustworthy completion | Any | Interrupted / execution_interrupted |
| Completed successfully | Satisfied | Succeeded |
| Execution not needed | Satisfied | AlreadySatisfied |
| Completed/skipped | Not satisfied | Failed / postcondition_failed |
| Completed/skipped | Cannot verify | Unverified / verification_unavailable |
| Unknown execution evidence | Any | Unverified / execution_unknown |

Verification includes the persisted selection and requested target, not just
presence. For Install, the selected detection check's requirements are authoritative
as defined above; top-level catalog `version` is not a verification threshold.
Presence alone does not prove that a required version, hash, or dependency
requirement is met. For Remove, absence and cleared local
install selection must be established, without a persistent local uninstall entry.
A cache write is not verification. A legacy “not needed” return alone is not
verification. Selection persistence errors are execution failures.

Stage 3 will carry diagnostic detail (underlying error, failing dependency,
installer/script stage) alongside the stable result classification. Do not turn
an installer error into success merely because the app is now present. No-op
adoption/withdrawal still persists the requested install selection (or its
withdrawal) before reporting success.

## Execution-scope investigation and activation gate

The preference is item-only work, but a direct call to an installer is not an
adequate replacement for `managedRun`. Source review establishes these couplings:

| Current behavior | Risk that narrowed execution must address |
| --- | --- |
| `cmd/gorilla/managed_run.go` configures downloads, initializes logging/cache, resolves manifests and catalogs, starts/ends reporting, and cleans the cache | Bypassing this setup can use stale configuration, omit policy/catalog inputs, or lose lifecycle/reporting work. Reuse a shared run lifecycle. |
| The same function processes installs, then uninstalls, then updates | An unrelated pending change may affect shared dependencies or a package's preconditions. Declared dependency/policy interactions must be tested; arbitrary package scripts can have undocumented assumptions. |
| `pkg/process.Installs` processes direct dependencies and then the target | Calling only `installer.Install` skips dependency handling. Current processing also skips missing dependencies and ignores returned failure strings; a full run does not guarantee dependency success either. |
| `pkg/report` uses global report state and one `GorillaReport.json`; Start does not clear item arrays | A partial run needs honest scope/result reporting and per-run isolation. It must not pretend to be a full inventory/convergence run. This shared-state concern already exists for repeated full runs. |
| `pkg/status` caches registry entries globally without a production invalidation path | Fresh post-install observations need explicit refresh/invalidation in the shared Go implementation. Running the entire pipeline again does not itself fix this. |
| `pkg/service` serializes commands and separately schedules startup/periodic full runs | Preserve this ordering and schedule; catalog reads/observations must not race shared state or starve managed work. |

I found no end-of-run commit/transaction that makes unrelated installs, removals,
and updates intrinsically mandatory after every optional action. That is a
source-review conclusion, not proof of equivalence for every catalog or script.

Before activating item-only execution in stage 3:

- Extract/reuse the common Go lifecycle and catalog/status/execution code; do not
  build a parallel UI-specific installer path.
- Test the requested app with dependencies, missing/failed dependencies, pending
  administrator install/uninstall/update policy, and another selected dependent.
- Verify configuration, reporting scope, cache cleanup, and fresh status across
  consecutive full and item-only runs.
- Verify one-time Remove followed by external reinstall and a scheduled run:
  user removal must not be enforced again. Keep administrator policy tests separate.
- Demonstrate that skipped unrelated work still runs on the normal schedule.

If these checks reveal a necessary broader scope, document the concrete failing
scenario and settle it before activation. Stage 1 records the preference and gate,
not an unconditional switch away from full runs.

## Protocol and CLI transition

The proposed payload examples are in [app-catalog-v2-examples.ndjson](app-catalog-v2-examples.ndjson).
They are design fixtures, not requests the current service accepts. The existing
[v1 examples](protocol-v0-examples.ndjson) describe the still-active protocol.

- Retain the canonical pipe and newline-delimited JSON envelopes/correlation IDs.
- Use envelope version `v2` for the changed list/result semantics. Unsupported
  versions must fail clearly; do not silently downgrade to v1 placeholder state.
- ListOptionalInstalls returns the new item snapshots with service-owned actions.
- InstallItem/RemoveItem accept an item name plus a stable client mutation ID.
  Acceptance means persisted intent/queued work, not installation success.
- StreamOperationStatus returns acknowledgement followed by operation snapshots
  until Completed, including the result. Reconnection may replay snapshots;
  clients must not apply a stale phase over a terminal result.
- Existing CLI grammar remains `-S ListOptionalInstalls`, `-S InstallItem:<item>`,
  `-S RemoveItem:<item>`, and `-S StreamOperationStatus:<operationId>`.
- On activation, update the Go CLI mapper, .NET transport/validation, cache schema,
  and protocol examples together. List output should include observed state and
  target/installed version distinctions; mutation output must say accepted/queued
  and provide the ID; the stream command must consume through terminal result.
- A request rejection is an envelope Error with a stable policy/validation code.
  A failure after acceptance is a completed operation with a failed result.
- CLI exits: 0 for accepted mutations and verified successful/no-op terminal
  results; nonzero for rejection, failure, interruption, or unverified outcome.
  An accepted mutation's exit code does not claim installer success.
- Do not deserialize old cache records as these new snapshots. Use a versioned
  cache and discard incompatible data in favor of a refresh.

No new v2 endpoint is exposed in stage 1. Payload validation at activation must
reject unknown enum values, missing required fields, invalid timestamps, invalid
progress, inconsistent phase/result pairs, and mismatched operation/item identity.
The reference types and pure functions do not substitute for transport validation.

### Recovery contract for stage 4

Use a client-generated mutation ID separate from per-connection requestId. The
service must persist the ID, item, action, and acceptance state before acknowledging
and before execution can begin. Resolve duplicate IDs to the existing operation;
reject reuse for a different item/action. UI reopening can discover active work
through the list and look up retained operations or uncertain mutations. Add the
lookup CLI commands with those endpoints in stage 4; do not claim they exist now.

Use bounded service-owned storage under ProgramData. Start with a 24-hour retention
window and at most 512 completed operations, without evicting active work. An
expired ID returns unknown/expired, never synthetic success; the client refreshes
and explains uncertainty instead of automatically submitting again. A failed
journal write prevents acceptance. Reconcile the journal and local selection
after a crash so intent is not silently dropped or duplicated. A retained Remove
operation is an execution/history record, not recurring managed-uninstall policy.
Clear selection durably at acceptance so scheduled installs cannot race the
removal. Do not automatically retry a failed or ambiguously interrupted removal.

On service restart, reconcile unfinished operations before finalizing their
results. Without trustworthy completion evidence, finalize as Interrupted; do
not replay an old installer blindly or later rewrite that terminal result.
Normal scheduled convergence may later satisfy persistent selections as separate
work, with a new observation/result. Retention, lookup transport, and crash
reconciliation tests belong to stage 4.

## Stage boundary and validation

This PR defines the rules and tests pure decisions and shared data shapes. It
does not implement detection adapters, dependency resolution, new request
handling, journal storage, or UI rendering. Those remain tracked in #208.

Run `make verify` for this stage. Go policy/result cases cover conflicting policy,
selection versus presence, unavailable recipes, unknown/failed detection, active
operations, no-op outcomes, execution errors, and unverified completion. Shared
payload examples are also read by .NET to keep the future service/client shapes
aligned. Runtime activation requires Windows integration and the UI validation
levels appropriate to the follow-up change.
