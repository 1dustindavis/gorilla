//go:build windows

package service

import (
	"context"
	"fmt"

	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/installer"
)

func (sr *serviceRunner) executeManagedItemRun(ctx context.Context, action, itemName string, resp CommandResponse) (installer.Result, error) {
	runCfg := sr.cfg
	if resp.RunConfig != nil {
		runCfg = *resp.RunConfig
	}

	// Use the same serialization mutex as queued startup/periodic runs. The item
	// callback still executes the ordinary full managed lifecycle; this only
	// preserves the accepted item's execution result across that boundary.
	sr.execMutex.Lock()
	defer sr.execMutex.Unlock()

	if err := ctx.Err(); err != nil {
		return installer.Result{}, err
	}
	if err := reconcileServiceManagedInstalls(runCfg); err != nil {
		return installer.Result{}, fmt.Errorf("reconcile service-managed installs before requested run: %w", err)
	}
	if sr.managedItemRun != nil {
		return sr.managedItemRun(runCfg, itemName, action)
	}
	if err := sr.managedRun(runCfg); err != nil {
		return installer.Result{}, err
	}
	return installer.Result{}, nil
}
