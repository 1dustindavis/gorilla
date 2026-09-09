package process

import (
	"reflect"
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
