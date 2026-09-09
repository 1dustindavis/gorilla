package service

import (
	"github.com/1dustindavis/gorilla/pkg/appcatalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/installer"
)

// ManagedItemRunFunc executes the ordinary managed-run lifecycle while retaining
// execution evidence for one accepted App Catalog item. It does not imply
// item-only execution; production currently supplies a full-run implementation.
type ManagedItemRunFunc func(config.Configuration, string, string) (installer.Result, error)

func appCatalogAction(action string) appcatalog.Action {
	if action == actionRemoveItem {
		return appcatalog.RemoveAction
	}
	return appcatalog.InstallAction
}

func operationTerminalEvent(itemName, action string, result operationResultPayload) operationStatusEventPayload {
	state := "Failed"
	switch result.Outcome {
	case appcatalog.Succeeded, appcatalog.AlreadySatisfied:
		state = "Succeeded"
	case appcatalog.Interrupted:
		state = "Canceled"
	}

	event := operationStatusEventPayload{
		State:    state,
		Message:  result.Message,
		ItemName: itemName,
		Action:   appCatalogAction(action),
		Result:   &result,
	}
	if result.Outcome == appcatalog.Failed || result.Outcome == appcatalog.Unverified {
		event.ErrorCode = result.Code
		if result.DetailCode != "" {
			event.ErrorCode = result.DetailCode
		}
		event.ErrorMessage = result.Message
	}
	if result.Outcome == appcatalog.Interrupted {
		event.CanceledBy = "service"
	}
	return event
}

func managedRunFailureResult(err error) operationResultPayload {
	return operationResultPayload{
		Outcome:    appcatalog.Failed,
		Code:       "execution_failed",
		DetailCode: "managed_run_failed",
		Message:    err.Error(),
	}
}

func interruptedOperationResult() operationResultPayload {
	return operationResultPayload{
		Outcome:    appcatalog.Interrupted,
		Code:       "execution_interrupted",
		DetailCode: "service_canceled",
		Message:    "Operation was interrupted before managed execution completed",
	}
}
