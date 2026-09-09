package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/appcatalog"
)

func TestOperationTerminalEventCarriesStructuredResultWithoutSyntheticProgress(t *testing.T) {
	result := operationResultPayload{
		Outcome:    appcatalog.Unverified,
		Code:       "verification_unavailable",
		DetailCode: "check_failed",
		Message:    "Could not verify the postcondition",
	}
	event := operationTerminalEvent("Example", actionInstallItem, result)

	if event.State != "Failed" || event.ItemName != "Example" || event.Action != appcatalog.InstallAction || event.Result == nil {
		t.Fatalf("terminal event lost operation identity/result: %+v", event)
	}
	if event.Result.Outcome != appcatalog.Unverified || event.Result.Code != "verification_unavailable" {
		t.Fatalf("unexpected structured result: %+v", event.Result)
	}
	if event.ErrorCode != "check_failed" {
		t.Fatalf("legacy error code should preserve actionable detail, got %q", event.ErrorCode)
	}

	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "progressPercent") {
		t.Fatalf("unmeasured progress must be omitted from transitional v1 event: %s", encoded)
	}
}

func TestInterruptedOperationMapsToCanceledCompatibilityState(t *testing.T) {
	event := operationTerminalEvent("Example", actionRemoveItem, interruptedOperationResult())
	if event.State != "Canceled" || event.CanceledBy != "service" || event.Result == nil || event.Result.Outcome != appcatalog.Interrupted || event.Result.Code != "execution_interrupted" {
		t.Fatalf("unexpected interrupted event: %+v", event)
	}
}

func TestAppCatalogActionDoesNotInventUnknownIdentity(t *testing.T) {
	if got := appCatalogAction(""); got != "" {
		t.Fatalf("empty action was fabricated as %q", got)
	}
	if got := appCatalogAction("unexpected"); got != "" {
		t.Fatalf("unknown action was fabricated as %q", got)
	}
	if got := appCatalogAction(actionInstallItem); got != appcatalog.InstallAction {
		t.Fatalf("InstallItem mapped to %q", got)
	}
	if got := appCatalogAction(actionRemoveItem); got != appcatalog.RemoveAction {
		t.Fatalf("RemoveItem mapped to %q", got)
	}
}
