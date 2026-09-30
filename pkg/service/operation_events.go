package service

import "github.com/1dustindavis/gorilla/pkg/appcatalog"

func appCatalogAction(action string) appcatalog.Action {
	switch action {
	case actionInstallItem:
		return appcatalog.InstallAction
	case actionRemoveItem:
		return appcatalog.RemoveAction
	default:
		return ""
	}
}

func operationTerminalEvent(itemName, action string, result operationResultPayload) operationStatusEventPayload {
	return operationStatusEventPayload{
		State:    "Completed",
		Message:  result.Message,
		ItemName: itemName,
		Action:   appCatalogAction(action),
		Result:   &result,
	}
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
