# App Catalog accessibility and UI Automation contract

This document defines the stable Stage 7 accessibility and UI Automation surface for the WinUI App Catalog. It complements the product contract in issue #208 and describes UI behavior, not service or action-policy semantics.

## Keyboard and focus behavior

Gorilla uses normal WinUI keyboard behavior rather than a custom navigation engine.

- Shell controls, Catalog search, catalog items and their embedded actions, Activity navigation, Details controls, and Activity rows are ordinary keyboard-focusable WinUI controls.
- Arrow-key movement within `GridView` / `ListView` remains framework-owned.
- Activating a catalog item with Enter or Space opens Details through `GridView.ItemClick`.
- Embedded card action buttons are independent focus targets. Enter or Space invokes that button only; the action must not also activate the containing card.
- When an invoked card action is focused and starting the operation will disable or replace that control, focus moves to the containing catalog item so the user remains in the same logical app context instead of being sent to an unrelated shell control.
- Catalog item identity is always the canonical `ItemName`, including after `GridView` virtualization or container recycling.
- Catalog → Details gives predictable entry focus to `DetailsBackButton`. The control uses the truthful generic accessible name `Back` because Details can be entered from either Catalog or Activity.
- Details → Catalog restores focus to the originating `ItemName` when it still exists. Restoration waits for the matching virtualized container to be actually realized, with a bounded fallback to `CatalogSearchBox` only if the logical item disappears or never materializes.
- Catalog → Activity gives predictable entry focus to `ActivityBackButton`.
- Activity → Details records the originating `OperationId`; back navigation waits for that logical Activity row's actual container realization and restores focus to it when it still exists, otherwise to `ActivityBackButton`.
- Catalog reconciliation and operation-state presentation must not deliberately move focus merely because data or a live region changed. The focused-action handoff above is the narrow exception because the focused control itself is becoming unavailable.

## Accessible semantics and announcements

- Interactive controls have user-facing accessible names independent of glyphs or automation selectors.
- Headings use `AutomationProperties.HeadingLevel`; page titles are level 1 and subordinate headings retain their lower level.
- Disabled WinUI actions expose their disabled state through UI Automation rather than through custom text-only state.
- `ProgressRing` / `ProgressBar` controls expose standard WinUI progress semantics and have the accessible name `Operation progress` where they represent an app operation.
- Technical-detail disclosures are named as technical details; exception and protocol text remains inside the disclosure instead of becoming the primary accessible status.
- Live-region semantics belong to the smallest stable element containing the complete user-facing message. Parent cards/banners are not also live regions when a child status element announces the same transition.
- When visible layout splits a status across a coarse state/title and a detail/message, the live element's UIA name combines those fragments into one semantic announcement. This includes Details active operation state + message, Details terminal title + user explanation, and Activity state/outcome + detail.
- `LiveRegionAnnouncer` baselines existing content after the initial load/binding turn. Loading a page or realizing a virtualized row therefore does not announce historical content as though it were a new transition.
- If an enabled live element changes while it or any visual ancestor is collapsed, that update is silent. When the region later becomes effectively visible, its current semantic message is announced once. Subsequent `Text` or `AutomationProperties.Name` changes announce only when the complete semantic message actually changes.
- Routine operation state uses polite announcements. Blocking load/infrastructure failures use assertive announcements. A state update must not steal keyboard focus.

## Stable automation IDs

Existing IDs are compatibility surface and are preserved unless a separately reviewed semantic defect requires correction.

### Shell

- `CatalogFreshnessStatus`
- `CatalogRefreshProgress`
- `CatalogRefreshButton`
- `CatalogDegradedWarning`
- `CatalogDegradedWarningText`
- `CatalogTechnicalDetails`
- `CatalogTechnicalDetailsContent`
- `InfrastructureWarning`
- `InfrastructureWarningText`
- `InfrastructureTechnicalDetails`
- `InfrastructureTechnicalDetailsContent`
- `NavigationFrame`

### Catalog

- `HomeHeading`
- `ActivityNavigationButton`
- `CatalogSearchBox`
- `CatalogItems`
- item container: canonical protocol `ItemName`
- repeated role IDs within an item: `CatalogCard`, `CatalogDisplayName`, `CatalogObservation`, `CatalogVersion`, `CatalogOperation`, `CatalogOperationStatus`, `CatalogTerminalFeedback`, `PrimaryActionButton`, `SecondaryActionButton`
- `CatalogInitialLoading`, `CatalogLoadFailed`, `CatalogNoCachedData`, `SearchNoResults`, `CatalogEmpty`

Repeated child IDs intentionally do not incorporate display name, status text, version, or any other mutable presentation value. Tests first scope to the `ItemName` container.

### Details

- `DetailsBackButton`
- `AppDetailsRoot`
- `DetailsDisplayName`
- `DetailsDescription`
- `DetailsObservation`
- `DetailsAvailableVersion`
- `DetailsInstalledVersion`
- `DetailsActiveOperation`
- `DetailsActiveOperationAnnouncement`
- `DetailsLatestResult`
- `DetailsActionExplanation`
- `DetailsPrimaryAction`, `DetailsSecondaryAction`
- `DetailsUnavailable`

Operation-specific recovery/troubleshooting controls retain the existing `OperationId`-rooted IDs produced by the Core presentation model, including `DetailsFailureTitle-{OperationId}`, `DetailsRetry-{OperationId}`, `DetailsRetryUnavailable-{OperationId}`, and technical-detail IDs.

### Activity

- `ActivityPageRoot`
- `ActivityBackButton`
- `ActivityHeading`
- `CatalogNavigationButton`
- `ActivityItems`
- `ActivityEmptyState`

Activity row identity is rooted in immutable `OperationId`: `ActivityOperation-{OperationId}`. Its existing state/action/progress/recovery child IDs remain `OperationId`-rooted as well. The row UIA `HelpText` also carries the raw `OperationId` for diagnostic correlation.

## Test selector rules

- Prefer `AutomationId`, control type, enabled state, and logical identity over visible text, screen coordinates, or template layout.
- Visible text is asserted only when the wording itself is product behavior.
- Critical keyboard-boundary tests send keyboard input rather than invoking UIA `Invoke`.
- Existing pointer/invoke critical-path coverage remains valuable and is not replaced by Stage 7 keyboard coverage.
- Repeated child automation IDs are scoped to their logical container. Tab-traversal validation walks the focused action's UIA ancestors and proves it belongs to the exact `ItemName` card reached by the preceding Tab.
- Virtualized collections are tested by logical identity and actual realization, not by fixed dispatcher-turn assumptions or counts of currently realized UIA containers. The Windows UI suite includes deep Catalog and Activity cases that force recycling before Details → back restoration.
- Live-region validation covers transition events, complete semantic messages, and the absence of spurious events when pages open with retained historical content.
- At least one successful accessibility scenario retains an automation tree. Failures continue to retain screenshots, automation tree, logs, and process metadata through the existing Windows UI harness.
