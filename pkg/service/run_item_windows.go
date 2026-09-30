//go:build windows

package service

import (
	"context"
	"fmt"

	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	"github.com/1dustindavis/gorilla/pkg/installer"
)

// executeManagedItemOperation keeps requested execution and postcondition
// verification inside the same serialization boundary as every other managed
// run. This prevents a queued startup/periodic run from changing catalog state
// between the requested action and the observation used to classify its result.
func (sr *serviceRunner) executeManagedItemOperation(ctx context.Context, action, itemName string, resp CommandResponse) (operationResultPayload, error) {
	runCfg := sr.cfg
	if resp.RunConfig != nil {
		runCfg = *resp.RunConfig
	}

	sr.execMutex.Lock()
	defer sr.execMutex.Unlock()

	if err := ctx.Err(); err != nil {
		return operationResultPayload{}, err
	}
	if err := reconcileServiceManagedInstalls(runCfg); err != nil {
		return operationResultPayload{}, fmt.Errorf("reconcile service-managed installs before requested run: %w", err)
	}

	var execution installer.Result
	var details []optionalItemDetails
	var detailsErr error
	trigger := catalogSnapshotTargetedInstall

	if sr.managedItemRun != nil {
		result, err := sr.managedItemRun(runCfg, itemName, action)
		if err != nil {
			return operationResultPayload{}, err
		}
		execution = result.Execution
		if action == actionInstallItem {
			// Install executes against persistent service configuration, so its
			// prepared repository state is safe to reuse. Projection still performs
			// fresh post-mutation observation and a registry-cache reset.
			details, detailsErr = getOptionalItemDetailsFromManagedContext(result.ExecutionPrepared)
		} else {
			// Remove may execute against a temporary one-time-removal manifest.
			// Never publish that transient policy as persistent App Catalog truth.
			// One fresh projection from sr.cfg is shared by verification + snapshot.
			trigger = catalogSnapshotTargetedRemove
			details, detailsErr = getOptionalItemDetails(sr.cfg)
		}
	} else {
		if _, err := sr.managedRun(runCfg); err != nil {
			return operationResultPayload{}, err
		}
		// Fallback full runs may receive operation-scoped RunConfig. Reproject from
		// persistent configuration instead of trusting the returned prepared state.
		if action == actionRemoveItem {
			trigger = catalogSnapshotTargetedRemove
		}
		details, detailsErr = getOptionalItemDetails(sr.cfg)
	}

	verified := verifyManagedItemResultFromDetails(sr.cfg, action, itemName, execution, details, detailsErr)
	if detailsErr != nil {
		gorillalog.Warn("catalog projection failed:", "trigger=", trigger, "error=", detailsErr)
		return verified, nil
	}
	if err := sr.publishCatalogSnapshot(details, sr.cfg, trigger); err != nil {
		// The in-memory candidate was already published; only persistence failed.
		gorillalog.Warn("catalog snapshot persistence failed after targeted operation:", err)
	}
	return verified, nil
}
