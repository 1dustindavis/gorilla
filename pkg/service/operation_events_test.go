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

	if event.State != "Completed" || event.ItemName != "Example" || event.Action != appcatalog.InstallAction || event.Result == nil {
		t.Fatalf("terminal event lost operation identity/result: %+v", event)
	}
	if event.Result.Outcome != appcatalog.Unverified || event.Result.Code != "verification_unavailable" {
		t.Fatalf("unexpected structured result: %+v", event.Result)
	}

	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	encodedText := string(encoded)
	if strings.Contains(encodedText, "progressPercent") {
		t.Fatalf("unmeasured progress must be omitted from event: %s", encodedText)
	}
	for _, legacyField := range []string{"errorCode", "errorMessage", "canceledBy"} {
		if strings.Contains(encodedText, legacyField) {
			t.Fatalf("legacy terminal field %q must not be emitted: %s", legacyField, encodedText)
		}
	}
}

func TestInterruptedOperationUsesCompletedWithStructuredOutcome(t *testing.T) {
	event := operationTerminalEvent("Example", actionRemoveItem, interruptedOperationResult())
	if event.State != "Completed" || event.Result == nil || event.Result.Outcome != appcatalog.Interrupted || event.Result.Code != "execution_interrupted" {
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
