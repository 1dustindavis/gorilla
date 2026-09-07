# App Catalog state and action contract

Stage 1 of [#208](https://github.com/1dustindavis/gorilla/issues/208).

This document records the decisions proposed for the completed optional-software
UI. `pkg/appcatalog` implements the pure action/result decisions; Client's
`AppCatalog` namespace defines matching payload records. Shared examples in
`pkg/appcatalog/testdata/contract.json` are exercised by Go and .NET tests.

**Implementation boundary:** the running service, CLI, cache, and WinUI app still
use v1. These types do not change current behavior or fix current placeholder
responses. Stages 2–3 supply real observations, enforce policy, and connect item
results to execution. Stage 4 supplies durable tracking/recovery. Enable v2 only
when service, CLI, and UI can use the complete contract together; do not label
v1 data as v2 or infer new state from its placeholder fields.

## Product decisions

- Show only effective `optional_installs` entries in App Catalog. Keep entries
  with invalid metadata visible with an explanation, but do not allow actions.
- Install persists the item in the device's local `managed_installs`, removes a
  local removal selection, and requests convergence to the catalog target.
- Remove withdraws that install selection and persists `managed_uninstalls`.
  This is a persistent removal request, not just forgetting the installation.
- Optional apps selected for installation update automatically. The first UI has
  no separate Update action. An unselected installed app can be adopted through
  Install; a selected app that is absent can be retried through Install.
- Administrator policy takes precedence over user selection. Conflicting
  administrator instructions are reported rather than resolved by execution order.
- Optional requests use the same serialized executor as scheduled work but run
  only the requested item and its necessary dependencies. They do not initiate
  an unrelated full managed run. Scheduled runs still converge all managed items.
- Managed-update notifications, installer cancellation, and automatic dependency
  garbage collection are outside this scope.

These are reviewable design decisions for this PR. They are not descriptions of
the old service's behavior: it currently schedules a full managed run per request
and does not enforce these action rules.

## Three independent kinds of state

| Kind | Meaning | Examples |
| --- | --- | --- |
| Observation | What fresh detection established on the device | Absent, Installed, UpdateAvailable, Unknown, DetectionFailed |
| Policy/selection | What administrators require and what the user selected | Required install, required removal, local Install/Remove/None |
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
| UpdateAvailable | Presence plus a valid version comparison establishes an older installed version | That an installer has started |
| Unknown | No check, not checked yet, unsupported or ambiguous evidence | Absence or an installation failure |
| DetectionFailed | An attempted check failed to execute/read/parse reliably | Absence or an installation failure |

- `targetVersion` comes from catalog `version`; `installedVersion` is observed
  evidence. Either can be null. Never copy the target into the installed version.
- `checkedAtUtc` is the observation/attempt time, not response construction time.
  It is null before an attempt. `detailCode` explains uncertainty or failed detection.
- Installed with an unknown version is valid. Do not label it “up to date.”
- A newer installed version is not an available update or a reason to downgrade.
- Hash/repair requirements are not version-based update availability.

### Detection adapters to implement in stage 2

Reuse Gorilla's check precedence and catalog resolution, but introduce explicit
observations rather than repurposing `CheckStatus`'s action-needed boolean.

| Check | Observation rules |
| --- | --- |
| Registry | Establish presence from matching uninstall entries; expose a parseable observed version when available. A registry read failure is DetectionFailed. Ambiguous matches cannot justify a version claim. |
| File | A readable required file establishes evidence; all required files present can establish Installed. All absent can establish Absent. Partial presence is Unknown. Access errors are DetectionFailed. Do not invent one app version from conflicting file versions. |
| AppX/MSIX | Observe provisioned package presence/version, matching Gorilla's machine-level scope. Do not present it as proof of launchability for each user's registration. |
| Legacy script | Existing exit codes describe whether action is needed, not necessarily presence or version. Do not translate zero into Absent or nonzero into Installed. Keep Unknown unless an explicit observation contract can establish presence; process-start/read errors are DetectionFailed. |
| Missing/unsupported check | Unknown with an explanation. |

Stage 2 must settle any additional script observation schema alongside examples
and tests. This plan intentionally disables actions when presence is Unknown or
DetectionFailed; a legacy script-only app may therefore remain unavailable in the
new UI until reliable observation is supplied. CLI/scheduled legacy behavior is
not changed by this stage. Revisit this explicit conservative choice if the
desired product behavior is to offer an unverified action instead.

## Action policy

The service owns these decisions. Core displays the supplied allowed/reason pair;
it does not duplicate policy. Evaluate against trusted, current data at request
acceptance and again before execution. Cached allowed actions are not authority.

Resolve `requiredInstall` and `requiredUninstall` from administrator-controlled
manifests, excluding the service-generated selection manifest. Local selection
is separately `None`, `Install`, or `Remove`; conflicting local entries are invalid.
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
this reconciliation in stages 2–3 with scheduled-run regression tests.

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
presence. For Install, presence alone does not prove that a required version,
hash, or dependency requirement is met. For Remove, absence and removal selection
must be established. A cache write is not verification. A legacy “not needed”
return alone is not verification. Selection persistence errors are execution failures.

Stage 3 will carry diagnostic detail (underlying error, failing dependency,
installer/script stage) alongside the stable result classification. Do not turn
an installer error into success merely because the app is now present. No-op
adoption/withdrawal still persists the requested selection before reporting success.

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
after a crash so intent is not silently dropped or duplicated.

On service restart, unfinished operations become Interrupted until reconciliation
establishes evidence; do not replay an old installer blindly. Normal scheduled
convergence may later satisfy persistent selections as separate work. Retention,
lookup transport, and crash reconciliation tests belong to stage 4.

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
