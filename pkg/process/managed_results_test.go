package process

import (
	"reflect"
	"strings"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/installer"
)

func TestManagedInstallResultsPreservesLegacyDependencyBehavior(t *testing.T) {
	previous := installerInstallResult
	t.Cleanup(func() { installerInstallResult = previous })

	catalogs := map[int]map[string]catalog.Item{1: {
		"ParentA": {
			DisplayName:  "ParentA",
			Dependencies: []string{"Child"},
			Installer:    catalog.InstallerItem{Type: "msi", Location: "parent-a.msi"},
		},
		"ParentB": {
			DisplayName:  "ParentB",
			Dependencies: []string{"Child"},
			Installer:    catalog.InstallerItem{Type: "msi", Location: "parent-b.msi"},
		},
		"Child": {
			DisplayName:  "Child",
			Dependencies: []string{"Grandchild"},
			Installer:    catalog.InstallerItem{Type: "msi", Location: "child.msi"},
		},
		"Grandchild": {
			DisplayName: "Grandchild",
			Installer:   catalog.InstallerItem{Type: "msi", Location: "grandchild.msi"},
		},
	}}

	var executed []string
	installerInstallResult = func(item catalog.Item, action, _, _ string, _ bool) installer.Result {
		executed = append(executed, item.DisplayName)
		return installer.Result{ItemName: item.DisplayName, Action: action, Outcome: installer.OutcomeSucceeded}
	}

	results := ManagedInstallResults([]string{"ParentA", "ParentB"}, catalogs, "", "", false)

	wantExecuted := []string{"Child", "ParentA", "Child", "ParentB"}
	if !reflect.DeepEqual(executed, wantExecuted) {
		t.Fatalf("result-aware managed install behavior changed: got %v, want %v", executed, wantExecuted)
	}
	wantResultItems := []string{"Child", "ParentA", "Child", "ParentB"}
	gotResultItems := make([]string, 0, len(results))
	for _, result := range results {
		gotResultItems = append(gotResultItems, result.ItemName)
	}
	if !reflect.DeepEqual(gotResultItems, wantResultItems) {
		t.Fatalf("result identities changed: got %v, want %v", gotResultItems, wantResultItems)
	}
}

func TestManagedInstallResultsReportsDependencyFailureWithoutChangingExecution(t *testing.T) {
	previous := installerInstallResult
	t.Cleanup(func() { installerInstallResult = previous })

	catalogs := map[int]map[string]catalog.Item{1: {
		"Parent": {
			DisplayName:  "Parent",
			Dependencies: []string{"Child"},
			Installer:    catalog.InstallerItem{Type: "msi", Location: "parent.msi"},
		},
		"Child": {
			DisplayName: "Child",
			Installer:   catalog.InstallerItem{Type: "msi", Location: "child.msi"},
		},
	}}

	var executed []string
	installerInstallResult = func(item catalog.Item, action, _, _ string, _ bool) installer.Result {
		executed = append(executed, item.DisplayName)
		if item.DisplayName == "Child" {
			return installer.Result{ItemName: item.DisplayName, Action: action, Outcome: installer.OutcomeFailed, ErrorCode: "download_failed", Message: "download error"}
		}
		return installer.Result{ItemName: item.DisplayName, Action: action, Outcome: installer.OutcomeSucceeded}
	}

	results := ManagedInstallResults([]string{"Parent"}, catalogs, "", "", false)
	if !reflect.DeepEqual(executed, []string{"Child", "Parent"}) {
		t.Fatalf("legacy execution changed after dependency failure: %v", executed)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	parent := results[1].Result
	if parent.Outcome != installer.OutcomeFailed || parent.ErrorCode != "dependency_failed" {
		t.Fatalf("parent result did not preserve dependency failure: %+v", parent)
	}
	if !strings.Contains(parent.Message, "Child") || !strings.Contains(parent.Message, "download_failed") {
		t.Fatalf("parent dependency diagnostic is incomplete: %q", parent.Message)
	}
}

func TestManagedInstallResultsReportsMissingDependencyWithoutSkippingParent(t *testing.T) {
	previous := installerInstallResult
	t.Cleanup(func() { installerInstallResult = previous })

	catalogs := map[int]map[string]catalog.Item{1: {
		"Parent": {
			DisplayName:  "Parent",
			Dependencies: []string{"Missing"},
			Installer:    catalog.InstallerItem{Type: "msi", Location: "parent.msi"},
		},
	}}

	var executed []string
	installerInstallResult = func(item catalog.Item, action, _, _ string, _ bool) installer.Result {
		executed = append(executed, item.DisplayName)
		return installer.Result{ItemName: item.DisplayName, Action: action, Outcome: installer.OutcomeSucceeded}
	}

	results := ManagedInstallResults([]string{"Parent"}, catalogs, "", "", false)
	if !reflect.DeepEqual(executed, []string{"Parent"}) {
		t.Fatalf("legacy missing-dependency execution changed: %v", executed)
	}
	if len(results) != 2 || results[0].ItemName != "Missing" || results[0].Result.ErrorCode != "invalid_dependency" {
		t.Fatalf("missing dependency result not preserved: %+v", results)
	}
	parent := results[1].Result
	if parent.Outcome != installer.OutcomeFailed || parent.ErrorCode != "dependency_failed" {
		t.Fatalf("parent did not report missing dependency: %+v", parent)
	}
}

func TestManagedActionResultsPreservesLegacyUpdateAndUninstallBoundary(t *testing.T) {
	previous := installerInstallResult
	t.Cleanup(func() { installerInstallResult = previous })

	// Deliberately omit installer/uninstaller metadata. The newer item-scoped
	// UpdateResults/UninstallResults APIs reject this shape as non-actionable, but
	// the historical managed-run loops resolve the item and still invoke the
	// installer boundary. This adapter must preserve that legacy behavior.
	catalogs := map[int]map[string]catalog.Item{1: {
		"Legacy": {DisplayName: "Legacy"},
	}}

	var actions []string
	installerInstallResult = func(item catalog.Item, action, _, _ string, _ bool) installer.Result {
		actions = append(actions, action+":"+item.DisplayName)
		return installer.Result{ItemName: item.DisplayName, Action: action, Outcome: installer.OutcomeSucceeded}
	}

	updates := ManagedActionResults([]string{"Missing", "Legacy"}, "update", catalogs, "", "", false)
	uninstalls := ManagedActionResults([]string{"Legacy", "Missing"}, "uninstall", catalogs, "", "", false)

	if !reflect.DeepEqual(actions, []string{"update:Legacy", "uninstall:Legacy"}) {
		t.Fatalf("legacy action execution changed: %v", actions)
	}
	if len(updates) != 1 || updates[0].ItemName != "Legacy" || updates[0].Result.Action != "update" {
		t.Fatalf("unexpected managed update results: %+v", updates)
	}
	if len(uninstalls) != 1 || uninstalls[0].ItemName != "Legacy" || uninstalls[0].Result.Action != "uninstall" {
		t.Fatalf("unexpected managed uninstall results: %+v", uninstalls)
	}
}
