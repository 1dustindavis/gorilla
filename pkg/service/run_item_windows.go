//go:build windows

package service

import (
	"context"
	"fmt"

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
	if sr.managedItemRun != nil {
		result, err := sr.managedItemRun(runCfg, itemName, action)
		if err != nil {
			return operationResultPayload{}, err
		}
		execution = result
	} else {
		if err := sr.managedRun(runCfg); err != nil {
			return operationResultPayload{}, err
		}
	}

	// Verify against the persistent service configuration rather than any
	// temporary one-shot removal manifest supplied only for this convergence.
	return verifyManagedItemResult(sr.cfg, action, itemName, execution), nil
}
