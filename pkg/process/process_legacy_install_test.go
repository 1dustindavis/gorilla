package process

import (
	"reflect"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
)

func TestInstallsPreservesLegacyDirectDependencyBehavior(t *testing.T) {
	previous := installerInstall
	t.Cleanup(func() { installerInstall = previous })

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
	installerInstall = func(item catalog.Item, _, _, _ string, _ bool) string {
		executed = append(executed, item.DisplayName)
		return ""
	}

	Installs([]string{"ParentA", "ParentB"}, catalogs, "", "", false)

	// Legacy managed processing handles only direct dependencies and does not
	// deduplicate a shared dependency across independently selected items.
	want := []string{"Child", "ParentA", "Child", "ParentB"}
	if !reflect.DeepEqual(executed, want) {
		t.Fatalf("legacy install behavior changed: got %v, want %v", executed, want)
	}
}
