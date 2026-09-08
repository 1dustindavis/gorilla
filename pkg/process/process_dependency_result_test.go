package process

import (
	"reflect"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/installer"
)

func TestInstallResultsReusesSharedDependencyFailure(t *testing.T) {
	previous := installerInstallResult
	t.Cleanup(func() { installerInstallResult = previous })

	catalogs := map[int]map[string]catalog.Item{1: {
		"ParentA": {
			DisplayName:  "ParentA",
			Dependencies: []string{"Shared"},
			Installer:    catalog.InstallerItem{Type: "msi", Location: "parent-a.msi"},
		},
		"ParentB": {
			DisplayName:  "ParentB",
			Dependencies: []string{"Shared"},
			Installer:    catalog.InstallerItem{Type: "msi", Location: "parent-b.msi"},
		},
		"Shared": {
			DisplayName: "Shared",
			Installer:   catalog.InstallerItem{Type: "msi", Location: "shared.msi"},
		},
	}}

	var executed []string
	installerInstallResult = func(item catalog.Item, action, _, _ string, _ bool) installer.Result {
		executed = append(executed, item.DisplayName)
		if item.DisplayName == "Shared" {
			return installer.Result{
				ItemName:  item.DisplayName,
				Action:    action,
				Outcome:   installer.OutcomeFailed,
				ErrorCode: "installer_failed",
				Message:   "shared dependency failed",
			}
		}
		return installer.Result{ItemName: item.DisplayName, Action: action, Outcome: installer.OutcomeSucceeded}
	}

	results := InstallResults([]string{"ParentA", "ParentB"}, catalogs, "", "", false)

	if want := []string{"Shared"}; !reflect.DeepEqual(executed, want) {
		t.Fatalf("dependents executed after shared dependency failure: got %v, want %v", executed, want)
	}

	if len(results) != 3 {
		t.Fatalf("unexpected results: %+v", results)
	}
	if results[0].ItemName != "Shared" || results[0].Result.ErrorCode != "installer_failed" {
		t.Fatalf("shared dependency failure was not retained: %+v", results)
	}
	if results[1].ItemName != "ParentA" || results[1].Result.ErrorCode != "dependency_failed" {
		t.Fatalf("first parent was not blocked by shared dependency failure: %+v", results)
	}
	if results[2].ItemName != "ParentB" || results[2].Result.ErrorCode != "dependency_failed" {
		t.Fatalf("second parent was not blocked by cached shared dependency failure: %+v", results)
	}
}
