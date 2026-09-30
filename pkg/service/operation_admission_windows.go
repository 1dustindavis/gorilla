//go:build windows

package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/1dustindavis/gorilla/pkg/gorillalog"
)

// executeCommandWithAdmission is the service-side authority for App Catalog
// mutation admission. The admission mutex covers duplicate/conflict checks,
// the persistent selection mutation, and operation registration so another UI
// instance cannot observe a half-admitted mutation.
func (sr *serviceRunner) executeCommandWithAdmission(cmd Command) (CommandResponse, error) {
	if cmd.Action == actionRun {
		return sr.executeManagedRunCommand(cmd)
	}
	if cmd.Action != actionInstallItem && cmd.Action != actionRemoveItem {
		return executeCommand(sr.cfg, cmd, sr.managedRun)
	}

	mutationID := strings.TrimSpace(cmd.MutationID)
	if mutationID == "" {
		return CommandResponse{}, errors.New("app catalog mutation requires mutationId")
	}

	sr.admissionMu.Lock()
	defer sr.admissionMu.Unlock()

	if existing, ok := sr.operationForMutation(mutationID); ok {
		if !operationIdentityMatches(existing, cmd.Items[0], cmd.Action) {
			return CommandResponse{}, actionDeniedError{"mutation_id_conflict"}
		}
		return CommandResponse{
			Status:          "ok",
			OperationID:     existing.operationID,
			ReusedOperation: true,
		}, nil
	}

	if sr.hasActiveOperationForItem(cmd.Items[0]) {
		return CommandResponse{}, actionDeniedError{"operation_active"}
	}

	resp, err := executeCommand(sr.cfg, cmd, sr.managedRun)
	if err != nil {
		return CommandResponse{}, err
	}

	// Registration happens before admission returns to the pipe handler. This is
	// what makes an acknowledgement retry safe: once the mutation side effect is
	// accepted, the same mutationId can always resolve back to this operation for
	// the remainder of the service lifetime/retention window.
	sr.registerTrackedOperation(resp.OperationID, cmd.Items[0], cmd.Action)
	sr.operationsMu.Lock()
	sr.mutationOperations[mutationID] = resp.OperationID
	sr.operationsMu.Unlock()

	return resp, nil
}

func (sr *serviceRunner) executeManagedRunCommand(cmd Command) (CommandResponse, error) {
	runCfg := sr.cfg
	persistent := cmd.RunConfig == nil
	if cmd.RunConfig != nil {
		runCfg = *cmd.RunConfig
	}
	if err := reconcileServiceManagedInstalls(runCfg); err != nil {
		return CommandResponse{}, fmt.Errorf("reconcile App Catalog install selections: %w", err)
	}
	result, err := sr.managedRun(runCfg)
	if err != nil {
		return CommandResponse{Status: "ok"}, err
	}
	if !persistent {
		// An operation-scoped full run may include transient policy. Targeted
		// execution performs the persistent post-operation projection instead.
		return CommandResponse{Status: "ok"}, nil
	}

	details, projectionErr := getOptionalItemDetailsFromManagedContext(result.Prepared)
	if projectionErr != nil {
		gorillalog.Warn("catalog projection failed:", "trigger=", catalogSnapshotManagedRun, "error=", projectionErr)
		return CommandResponse{Status: "ok"}, nil
	}
	if publishErr := sr.publishCatalogSnapshot(details, sr.cfg, catalogSnapshotManagedRun); publishErr != nil {
		// publishCatalogSnapshot swaps the complete in-memory candidate before
		// persistence, so an on-disk failure must not fail managed convergence.
		gorillalog.Warn("catalog snapshot persistence failed after managed run:", publishErr)
	}
	return CommandResponse{Status: "ok"}, nil
}

type admittedOperation struct {
	operationID string
	itemName    string
	action      string
}

func (sr *serviceRunner) operationForMutation(mutationID string) (admittedOperation, bool) {
	sr.operationsMu.Lock()
	defer sr.operationsMu.Unlock()

	opID, ok := sr.mutationOperations[mutationID]
	if !ok {
		return admittedOperation{}, false
	}
	op, ok := sr.operations[opID]
	if !ok || len(op.events) == 0 {
		delete(sr.mutationOperations, mutationID)
		return admittedOperation{}, false
	}
	first := op.events[0]
	action := ""
	switch first.Action {
	case "Install":
		action = actionInstallItem
	case "Remove":
		action = actionRemoveItem
	}
	return admittedOperation{
		operationID: opID,
		itemName:    first.ItemName,
		action:      action,
	}, true
}

func (sr *serviceRunner) hasActiveOperationForItem(itemName string) bool {
	sr.operationsMu.Lock()
	defer sr.operationsMu.Unlock()

	for _, op := range sr.operations {
		if op.done || len(op.events) == 0 {
			continue
		}
		if strings.EqualFold(op.events[0].ItemName, itemName) {
			return true
		}
	}
	return false
}

func operationIdentityMatches(op admittedOperation, itemName, action string) bool {
	return strings.EqualFold(op.itemName, itemName) && op.action == action
}
