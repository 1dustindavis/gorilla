# App Catalog

![Gorilla App Catalog](https://github.com/user-attachments/assets/5defc532-6d4e-4961-8c90-3b4648e3c650)

App Catalog is Gorilla's Windows interface for optional software. It shows software assigned through `optional_installs` and lets users request the actions currently permitted by Gorilla policy and device state.

## What appears in App Catalog

App Catalog shows effective optional-software assignments from Gorilla's configured manifests/catalogs. Cards and Details can include the app name, description, available version, detected installed state/version when Gorilla can determine them, and the actions currently available.

Installed-state information and action availability come from the Gorilla service. App Catalog does not independently inspect the machine or decide whether an action is allowed.

## Actions

The exact action shown depends on current observation, policy, selection, and whether work is already active.

- **Install** selects the optional app for ongoing Gorilla management and requests installation/convergence when needed. An already-installed but unselected app can also be adopted into managed installs when policy permits.
- **Update** may be presented when the current action model exposes an install/convergence action for software with an available update; Gorilla still uses the service's current action decision rather than a client-side version comparison.
- **Keep Installed** reflects the managed selection for software Gorilla should continue to keep installed/updated; it is not a separate installer replay.
- **Remove** clears the user's local managed-install selection and performs a one-time uninstall when required and permitted. It does not create a persistent user-managed uninstall rule. If the app is later installed again outside Gorilla, the old user Remove is not repeatedly enforced.

Administrator-required install/uninstall policy and dependency requirements can make an action unavailable. App Catalog shows the service-provided explanation rather than overriding those rules.

## Activity and results

**Activity** shows active and recently retained App Catalog operations with the app/action identity and current or final result. Closing and reopening App Catalog does not restart an in-progress service operation; while the Gorilla service remains running, the UI reconstructs Activity from the service's retained operation records.

Operation failures are presented in plain language. When useful for troubleshooting, **Technical details** exposes the underlying result code/message or infrastructure information without making raw protocol/exception text the primary user-facing explanation.

## Retry after failure

When a failed, unverified, or interrupted historical operation is still eligible under the app's **current** state and policy, App Catalog can offer **Retry**.

Retry is a new service-authorized action. It does not replay the old operation ID or blindly resubmit the old request. Gorilla rechecks current catalog/device/policy truth before accepting the new action, and the earlier failed operation remains in Activity as history.

## Refresh, freshness, and cached data

App Catalog loads usable cached catalog data quickly at startup when available, then requests fresh data from the Gorilla service.

The UI distinguishes live data from degraded/cached presentation:

- **Refresh** requests current catalog/device/action information from the service.
- Last-updated/freshness presentation shows when current data was obtained.
- If live refresh fails but a usable cache exists, cached software can remain visible with a stale/offline/degraded warning.
- Cached action availability is not authority for a new mutation; the service revalidates every Install/Remove/Retry request.
- If there is no usable cache and the service cannot load data, App Catalog shows explicit load-failed/no-cached-data state.
- If fresh service data loads but saving the cache fails, the fresh data remains usable and the cache problem is shown as degradation rather than a false service failure.

Loading, no optional software assigned, no search results, no cached data, and load failure are distinct states.

## Service dependency and installed package behavior

Install the versioned `gorilla-<version>.msix` from Gorilla releases, for example:

```powershell
Add-AppxPackage -Path .\gorilla-2.30.0.msix -ForceUpdateFromAnyVersion
```

The MSIX installs App Catalog and registers the Gorilla Windows service. The service runs with the privileges needed to evaluate policy/detection and perform software changes; the App Catalog UI normally runs as the signed-in user and communicates with that service over Gorilla's named pipe.

If the service is unavailable, App Catalog cannot authorize or start new software mutations. Cached catalog data may still be displayed when available, clearly marked as degraded/stale.

App Catalog requires Gorilla configuration that assigns one or more `optional_installs` entries to display software. Normal Gorilla scheduled management continues independently of whether the App Catalog window is open.

For implementation details and contributor validation, see [`gorilla-ui/ARCHITECTURE.md`](../gorilla-ui/ARCHITECTURE.md) and [`gorilla-ui/README.md`](../gorilla-ui/README.md).
