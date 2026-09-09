package service

import (
	"fmt"
	"slices"

	"github.com/1dustindavis/gorilla/pkg/appcatalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/installer"
	"github.com/1dustindavis/gorilla/pkg/manifest"
)

// operationResultPayload is the service boundary for a terminal App Catalog
// result. Code is the stable Stage 1 classification; DetailCode preserves the
// lower-level installer or detection reason without making clients parse Message.
type operationResultPayload struct {
	Outcome    appcatalog.Outcome `json:"outcome"`
	Code       string             `json:"code"`
	DetailCode string             `json:"detailCode,omitempty"`
	Message    string             `json:"message,omitempty"`
}

func verifyManagedItemResult(cfg config.Configuration, action, itemName string, execution installer.Result) operationResultPayload {
	details, err := getOptionalItemDetails(cfg)
	if err != nil {
		return operationResultPayload{
			Outcome:    appcatalog.Unverified,
			Code:       "verification_unavailable",
			DetailCode: "catalog_refresh_failed",
			Message:    fmt.Sprintf("Unable to refresh App Catalog state after operation: %v", err),
		}
	}
	item, found := findOptionalItem(details, itemName)
	selection, selectionErr := loadServiceLocalManifest(cfg)
	return classifyOperationResult(action, itemName, execution, item.Contract, found, selection, selectionErr)
}

func classifyOperationResult(action, itemName string, execution installer.Result, item appcatalog.Item, itemFound bool, selection manifest.Item, selectionErr error) operationResultPayload {
	if execution.Outcome == installer.OutcomeFailed {
		detail := execution.ErrorCode
		if detail == "" {
			detail = "execution_failed"
		}
		message := execution.Message
		if message == "" {
			message = "Requested work failed"
		}
		return operationResultPayload{Outcome: appcatalog.Failed, Code: "execution_failed", DetailCode: detail, Message: message}
	}

	if !itemFound {
		return operationResultPayload{
			Outcome:    appcatalog.Unverified,
			Code:       "verification_unavailable",
			DetailCode: "catalog_item_unavailable",
			Message:    "Requested item could not be resolved for post-operation verification",
		}
	}
	if selectionErr != nil {
		return operationResultPayload{
			Outcome:    appcatalog.Unverified,
			Code:       "verification_unavailable",
			DetailCode: "selection_read_failed",
			Message:    fmt.Sprintf("Unable to verify persisted App Catalog selection: %v", selectionErr),
		}
	}

	switch action {
	case actionInstallItem:
		if !slices.Contains(selection.Installs, itemName) {
			return operationResultPayload{
				Outcome:    appcatalog.Failed,
				Code:       "postcondition_failed",
				DetailCode: "install_selection_not_persisted",
				Message:    "Install selection was not persisted after the operation",
			}
		}
		if slices.Contains(selection.Uninstalls, itemName) {
			return operationResultPayload{
				Outcome:    appcatalog.Failed,
				Code:       "postcondition_failed",
				DetailCode: "persistent_uninstall_present",
				Message:    "Persistent local uninstall policy conflicts with the install selection",
			}
		}
		switch item.Observation.InstallRequirement {
		case appcatalog.RequirementSatisfied:
			if execution.Outcome == installer.OutcomeSucceeded {
				return operationResultPayload{Outcome: appcatalog.Succeeded, Message: "Install requirement is satisfied"}
			}
			if execution.Outcome == installer.OutcomeAlreadyCurrent || execution.Outcome == "" {
				return operationResultPayload{Outcome: appcatalog.AlreadySatisfied, Message: "Install requirement was already satisfied"}
			}
			return operationResultPayload{Outcome: appcatalog.Unverified, Code: "execution_unknown", Message: "Install requirement is satisfied but execution evidence is unknown"}
		case appcatalog.RequirementNotSatisfied:
			return operationResultPayload{
				Outcome:    appcatalog.Failed,
				Code:       "postcondition_failed",
				DetailCode: item.Observation.DetailCode,
				Message:    "Install requirement is still not satisfied after the operation",
			}
		default:
			detail := item.Observation.DetailCode
			if detail == "" {
				detail = "requirement_unknown"
			}
			return operationResultPayload{
				Outcome:    appcatalog.Unverified,
				Code:       "verification_unavailable",
				DetailCode: detail,
				Message:    "Install requirement could not be verified after the operation",
			}
		}

	case actionRemoveItem:
		if slices.Contains(selection.Installs, itemName) {
			return operationResultPayload{
				Outcome:    appcatalog.Failed,
				Code:       "postcondition_failed",
				DetailCode: "install_selection_not_cleared",
				Message:    "Install selection remains after the remove operation",
			}
		}
		if slices.Contains(selection.Uninstalls, itemName) {
			return operationResultPayload{
				Outcome:    appcatalog.Failed,
				Code:       "postcondition_failed",
				DetailCode: "persistent_uninstall_present",
				Message:    "Persistent local uninstall policy remains after the remove operation",
			}
		}
		switch item.Observation.State {
		case appcatalog.Absent:
			if execution.Outcome == installer.OutcomeSucceeded {
				return operationResultPayload{Outcome: appcatalog.Succeeded, Message: "Item is absent and local selection is cleared"}
			}
			if execution.Outcome == installer.OutcomeAlreadyCurrent || execution.Outcome == "" {
				return operationResultPayload{Outcome: appcatalog.AlreadySatisfied, Message: "Item was already absent and local selection is cleared"}
			}
			return operationResultPayload{Outcome: appcatalog.Unverified, Code: "execution_unknown", Message: "Remove postcondition is satisfied but execution evidence is unknown"}
		case appcatalog.Unknown, appcatalog.DetectionFailed:
			detail := item.Observation.DetailCode
			if detail == "" {
				detail = "absence_unknown"
			}
			return operationResultPayload{
				Outcome:    appcatalog.Unverified,
				Code:       "verification_unavailable",
				DetailCode: detail,
				Message:    "Absence could not be verified after the remove operation",
			}
		default:
			return operationResultPayload{
				Outcome:    appcatalog.Failed,
				Code:       "postcondition_failed",
				DetailCode: item.Observation.DetailCode,
				Message:    "Item is still present after the remove operation",
			}
		}
	default:
		return operationResultPayload{Outcome: appcatalog.Unverified, Code: "execution_unknown", DetailCode: "unsupported_action", Message: "Operation action is not supported for verification"}
	}
}
