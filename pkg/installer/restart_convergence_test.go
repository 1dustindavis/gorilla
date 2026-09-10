package installer

import (
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
)

func TestRestartConvergenceSkipsInstallerWhenDetectionIsSatisfied(t *testing.T) {
	previousStatus := statusCheckStatus
	previousInstall := installItemFunc
	t.Cleanup(func() {
		statusCheckStatus = previousStatus
		installItemFunc = previousInstall
	})

	statusChecks := 0
	installerCalls := 0
	statusCheckStatus = func(catalog.Item, string, string) (bool, error) {
		statusChecks++
		// A fresh service process performing normal managed convergence sees that
		// the selected install is already satisfied by the interrupted/previous run.
		return false, nil
	}
	installItemFunc = func(catalog.Item, string, string) (string, error) {
		installerCalls++
		return "", nil
	}

	result := InstallResult(
		catalog.Item{DisplayName: "Example"},
		"install",
		"https://example.invalid/",
		t.TempDir(),
		false,
	)

	if statusChecks != 1 {
		t.Fatalf("expected exactly one status check before convergence decision, got %d", statusChecks)
	}
	if installerCalls != 0 {
		t.Fatalf("expected satisfied restart convergence to skip installer, got %d calls", installerCalls)
	}
	if result.Outcome != OutcomeAlreadyCurrent {
		t.Fatalf("expected already-current outcome, got %+v", result)
	}
}
