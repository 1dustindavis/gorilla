package status

import (
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
)

// The future App Catalog observation adapter must preserve this authority.
// Exercise the shared check directly, without a second version comparator.
func TestCheckStatusRegistryVersionAuthority(t *testing.T) {
	previous := RegistryItems
	t.Cleanup(func() { RegistryItems = previous })
	RegistryItems = map[string]RegistryApplication{
		"example": {Name: "Example", Version: "1.7"},
	}

	for _, tc := range []struct {
		name           string
		catalogVersion string
		checkVersion   string
		wantAction     bool
	}{
		{"catalog newer but check satisfied", "2.0", "1.5", false},
		{"catalog older but check unsatisfied", "1.5", "2.0", true},
		{"check exactly satisfied", "2.0", "1.7", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := catalog.Item{
				DisplayName: "Example",
				Version:     tc.catalogVersion,
				Check: catalog.InstallCheck{
					Registry: catalog.RegCheck{Name: "Example", Version: tc.checkVersion},
				},
			}
			for _, action := range []string{"install", "update"} {
				got, err := CheckStatus(item, action, "")
				if err != nil {
					t.Fatal(err)
				}
				if got != tc.wantAction {
					t.Fatalf("%s: actionNeeded=%v, want %v (catalog=%s check=%s installed=1.7)",
						action, got, tc.wantAction, tc.catalogVersion, tc.checkVersion)
				}
			}
		})
	}
}

func TestObserveRegistryVersionAuthority(t *testing.T) {
	previous := RegistryItems
	t.Cleanup(func() { RegistryItems = previous })
	RegistryItems = map[string]RegistryApplication{
		"example": {Name: "Example", Version: "1.7"},
	}

	for _, tc := range []struct {
		catalogVersion string
		checkVersion   string
		wantState      ObservedState
	}{
		{"2.0", "1.5", Installed},
		{"1.5", "2.0", UpdateAvailable},
	} {
		item := catalog.Item{
			DisplayName: "Example", Version: tc.catalogVersion,
			Check: catalog.InstallCheck{Registry: catalog.RegCheck{Name: "Example", Version: tc.checkVersion}},
		}
		got, err := Observe(item, "install", "")
		if err != nil {
			t.Fatal(err)
		}
		if got.State != tc.wantState || got.InstalledVersion == nil || *got.InstalledVersion != "1.7" {
			t.Fatalf("catalog=%s check=%s: got %+v, want %s at 1.7", tc.catalogVersion, tc.checkVersion, got, tc.wantState)
		}
	}
}

func TestObserveDoesNotInventScriptPresence(t *testing.T) {
	previous := execCommand
	t.Cleanup(func() { execCommand = previous })
	execCommand = fakeExecCommand
	item := catalog.Item{
		DisplayName: "Script",
		Check: catalog.InstallCheck{
			Script: "exit 0",
			// A lower-priority file check must not replace or supplement
			// the selected script check.
			File: []catalog.FileCheck{{Path: "testdata/does-not-exist"}},
		},
	}
	got, err := Observe(item, "install", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != Unknown || !got.ActionNeeded || got.DetailCode != "script_requirement_not_satisfied" {
		t.Fatalf("script observation or precedence changed: %+v", got)
	}
}

func TestObserveFilePresenceDoesNotCallHashRepairAnUpdate(t *testing.T) {
	current, err := Observe(pathInstalled, "install", "")
	if err != nil {
		t.Fatal(err)
	}
	if current.State != Installed || current.ActionNeeded {
		t.Fatalf("matching file was not installed: %+v", current)
	}
	repair, err := Observe(pathNotInstalled, "install", "")
	if err != nil {
		t.Fatal(err)
	}
	if repair.State != Installed || !repair.ActionNeeded || repair.DetailCode != "non_version_requirement_unsatisfied" {
		t.Fatalf("hash repair was mislabeled: %+v", repair)
	}
	absent, err := Observe(pathMissing, "install", "")
	if err != nil {
		t.Fatal(err)
	}
	if absent.State != Absent || !absent.ActionNeeded {
		t.Fatalf("missing file was not absent: %+v", absent)
	}
}

func TestObserveAppxPresenceAndVersion(t *testing.T) {
	previous := execCommand
	t.Cleanup(func() { execCommand = previous })
	execCommand = fakeExecCommandAppx
	for _, tc := range []struct {
		item    catalog.Item
		state   ObservedState
		version string
	}{
		{appxCheckInstalled, Installed, "1.2.0"},
		{appxCheckOutdated, UpdateAvailable, "1.2.0"},
		{appxCheckNotInstalled, Absent, ""},
	} {
		got, err := Observe(tc.item, "install", "")
		if err != nil {
			t.Fatal(err)
		}
		if got.State != tc.state {
			t.Fatalf("%s: got %+v", tc.item.DisplayName, got)
		}
		if tc.version != "" && (got.InstalledVersion == nil || *got.InstalledVersion != tc.version) {
			t.Fatalf("%s: lost installed version: %+v", tc.item.DisplayName, got)
		}
	}
}

func TestObservePreservesOrderedMultiFileActionDecision(t *testing.T) {
	missingFirst := catalog.Item{Check: catalog.InstallCheck{File: []catalog.FileCheck{
		{Path: "testdata/missing-first"},
		{Path: "testdata/test_checkPath.msi", Hash: "invalid"},
	}}}
	for _, action := range []string{"update", "uninstall"} {
		got, err := Observe(missingFirst, action, "")
		if err != nil {
			t.Fatal(err)
		}
		if got.State != Unknown || got.ActionNeeded {
			t.Fatalf("%s changed legacy first-missing decision: %+v", action, got)
		}
	}
	presentFirst := catalog.Item{Check: catalog.InstallCheck{File: []catalog.FileCheck{
		{Path: "testdata/test_checkPath.msi"},
		{Path: "testdata/missing-second"},
	}}}
	got, err := Observe(presentFirst, "uninstall", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != Unknown || !got.ActionNeeded {
		t.Fatalf("uninstall lost legacy first-present decision: %+v", got)
	}
}
