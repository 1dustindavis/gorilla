package process

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/installer"
	"github.com/1dustindavis/gorilla/pkg/manifest"
)

var (
	origOsRemove = osRemove

	testCatalogs = map[int]map[string]catalog.Item{1: {
		"Chocolatey": {
			DisplayName:  "Chocolatey",
			Installer:    catalog.InstallerItem{Type: "msi", Location: "Chocolatey.msi"},
			Dependencies: []string{"TestUpdate1"},
		},
		"GoogleChrome": {
			DisplayName: "GoogleChrome",
			Installer:   catalog.InstallerItem{Type: "msi", Location: "GoogleChrome.msi"},
		},
		"TestInstall1": {
			DisplayName: "TestInstall1",
			Installer:   catalog.InstallerItem{Type: "msi", Location: "TestInstall1.msi"},
		},
		"TestInstall2": {
			DisplayName: "TestInstall2",
			Installer:   catalog.InstallerItem{Type: "msi", Location: "TestInstall2.msi"},
		},
		"AdobeFlash": {
			DisplayName: "AdobeFlash",
			Uninstaller: catalog.InstallerItem{Type: "msi", Location: "AdobeUninst.msi"},
		},
		"Chef Client": {
			DisplayName: "Chef Client",
			Installer:   catalog.InstallerItem{Type: "msi", Location: "chef.msi"},
		},
		"CanonDrivers": {
			DisplayName: "CanonDrivers",
			Installer:   catalog.InstallerItem{Type: "msi", Location: "TestInstall1.msi"},
		},
		"TestUninstall1": {
			DisplayName: "TestUninstall1",
			Uninstaller: catalog.InstallerItem{Type: "ps1", Location: "TestUninst2.ps1"},
		},
		"TestUninstall2": {
			DisplayName: "TestUninstall2",
			Uninstaller: catalog.InstallerItem{Type: "exe", Location: "TestUninst2.exe"},
		},
		"TestUpdate1": {
			DisplayName: "TestUpdate1",
			Installer:   catalog.InstallerItem{Type: "nupkg", Location: "TestUpdate1.nupkg"},
		},
		"TestUpdate2": {
			DisplayName: "TestUpdate2",
			Installer:   catalog.InstallerItem{Type: "ps1", Location: "TestUpdate2.ps1"},
		},
		"MissingInstallerType": {
			DisplayName: "MissingInstallerType",
			Installer:   catalog.InstallerItem{Location: "MissingInstallerType.msi"},
		},
		"MissingInstallerLocation": {
			DisplayName: "MissingInstallerLocation",
			Installer:   catalog.InstallerItem{Type: "msi"},
		},
		"TestMsixInstallOnly": {
			DisplayName: "TestMsixInstallOnly",
			Check:       catalog.InstallCheck{Appx: catalog.AppxCheck{Name: "TestPublisher.TestApp"}},
			Installer:   catalog.InstallerItem{Type: "msix", Location: "TestApp.msix"},
		},
		"TestMsixUninstall": {
			DisplayName: "TestMsixUninstall",
			Check:       catalog.InstallCheck{Appx: catalog.AppxCheck{Name: "TestPublisher.TestApp"}},
			Uninstaller: catalog.InstallerItem{Type: "msix"},
		},
	}}

	testInstalls   = []string{"Chocolatey", "GoogleChrome", "TestInstall1", "TestInstall2"}
	testUninstalls = []string{"AdobeFlash", "TestUninstall1", "TestUninstall2"}
	testUpdates    = []string{"Chef Client", "CanonDrivers", "TestUpdate1", "TestUpdate2"}

	actualRemovedFiles []string
)

func TestManifests(t *testing.T) {
	testManifests := []manifest.Item{
		{
			Name:       "example_manifest",
			Includes:   []string{"included_manifest"},
			Installs:   []string{"Chocolatey", "GoogleChrome"},
			Uninstalls: []string{"AdobeFlash"},
			Updates:    []string{"Chef Client", "CanonDrivers"},
		},
		{
			Name:       "included_manifest",
			Installs:   []string{"TestInstall1", "TestInstall2", "MissingInstallerType", "MissingInstallerLocation"},
			Uninstalls: []string{"TestUninstall1", "TestUninstall2"},
			Updates:    []string{"TestUpdate1", "TestUpdate2"},
		},
	}

	actualInstalls, actualUninstalls, actualUpdates := Manifests(testManifests, testCatalogs)
	if !reflect.DeepEqual(testInstalls, actualInstalls) {
		t.Fatalf("manifest installs: got %#v, want %#v", actualInstalls, testInstalls)
	}
	if !reflect.DeepEqual(testUninstalls, actualUninstalls) {
		t.Fatalf("manifest uninstalls: got %#v, want %#v", actualUninstalls, testUninstalls)
	}
	if !reflect.DeepEqual(testUpdates, actualUpdates) {
		t.Fatalf("manifest updates: got %#v, want %#v", actualUpdates, testUpdates)
	}
}

func TestFirstItemInvalidReturnsFalse(t *testing.T) {
	for _, name := range []string{"MissingInstallerType", "MissingInstallerLocation", "DoesNotExist"} {
		if _, ok := firstItem(name, testCatalogs); ok {
			t.Fatalf("expected %q to be skipped", name)
		}
	}

	item, ok := firstItem("Chocolatey", testCatalogs)
	if !ok || item.DisplayName != "Chocolatey" {
		t.Fatalf("unexpected item returned: %#v, ok=%v", item, ok)
	}
}

func TestFirstItemMsixNoLocationIsValid(t *testing.T) {
	item, ok := firstItem("TestMsixUninstall", testCatalogs)
	if !ok || item.DisplayName != "TestMsixUninstall" {
		t.Fatalf("expected msix uninstall item with no location to be valid: %#v, ok=%v", item, ok)
	}
}

func TestFirstItemMsixInstallerTypeIsValid(t *testing.T) {
	item, ok := firstItem("TestMsixInstallOnly", testCatalogs)
	if !ok || item.DisplayName != "TestMsixInstallOnly" {
		t.Fatalf("expected msix installer-only item to be valid for uninstall: %#v, ok=%v", item, ok)
	}
}

func TestInstallResultsExecutesTransitiveDependenciesOnce(t *testing.T) {
	previous := installerInstallResult
	t.Cleanup(func() { installerInstallResult = previous })

	catalogs := map[int]map[string]catalog.Item{1: {
		"Parent": {
			DisplayName:  "Parent",
			Dependencies: []string{"Child"},
			Installer:    catalog.InstallerItem{Type: "msi", Location: "parent.msi"},
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

	results := InstallResults([]string{"Parent", "Child"}, catalogs, "", "", false)
	if want := []string{"Grandchild", "Child", "Parent"}; !reflect.DeepEqual(executed, want) {
		t.Fatalf("unexpected execution order: got %v, want %v", executed, want)
	}
	if len(results) != 3 || results[2].ItemName != "Parent" || results[2].Result.Outcome != installer.OutcomeSucceeded {
		t.Fatalf("unexpected results: %+v", results)
	}
}

func TestInstallResultsFailsParentWhenDependencyIsUnsatisfiable(t *testing.T) {
	catalogs := map[int]map[string]catalog.Item{1: {
		"Parent": {
			DisplayName:  "Parent",
			Dependencies: []string{"Missing"},
			Installer:    catalog.InstallerItem{Type: "msi", Location: "parent.msi"},
		},
	}}

	results := InstallResults([]string{"Parent"}, catalogs, "", "", false)
	if len(results) != 2 || results[0].ItemName != "Missing" || results[0].Result.ErrorCode != "invalid_dependency" {
		t.Fatalf("missing dependency was not reported: %+v", results)
	}
	if results[1].ItemName != "Parent" || results[1].Result.ErrorCode != "dependency_failed" {
		t.Fatalf("parent was not blocked by dependency failure: %+v", results)
	}
}

func TestInstallResultsDetectsDependencyCycle(t *testing.T) {
	catalogs := map[int]map[string]catalog.Item{1: {
		"A": {DisplayName: "A", Dependencies: []string{"B"}, Installer: catalog.InstallerItem{Type: "msi", Location: "a.msi"}},
		"B": {DisplayName: "B", Dependencies: []string{"A"}, Installer: catalog.InstallerItem{Type: "msi", Location: "b.msi"}},
	}}

	results := InstallResults([]string{"A"}, catalogs, "", "", false)
	if len(results) != 2 || results[0].ItemName != "B" || results[0].Result.ErrorCode != "dependency_cycle" || results[1].ItemName != "A" || results[1].Result.ErrorCode != "dependency_cycle" {
		t.Fatalf("dependency cycle was not reported deterministically: %+v", results)
	}
}

func TestUninstallResultsKeepsFailureWithItsItem(t *testing.T) {
	previous := installerInstallResult
	t.Cleanup(func() { installerInstallResult = previous })
	catalogs := map[int]map[string]catalog.Item{1: {
		"Good": {DisplayName: "Good", Uninstaller: catalog.InstallerItem{Type: "msi", Location: "good.msi"}},
		"Bad":  {DisplayName: "Bad"},
	}}
	installerInstallResult = func(item catalog.Item, action, _, _ string, _ bool) installer.Result {
		return installer.Result{ItemName: item.DisplayName, Action: action, Outcome: installer.OutcomeSucceeded}
	}

	results := UninstallResults([]string{"Good", "Bad"}, catalogs, "", "", false)
	if len(results) != 2 || results[0].ItemName != "Good" || results[0].Result.Outcome != installer.OutcomeSucceeded {
		t.Fatalf("successful uninstall result was not retained: %+v", results)
	}
	if results[1].ItemName != "Bad" || results[1].Result.ErrorCode != "invalid_catalog_item" {
		t.Fatalf("invalid uninstall result was not retained: %+v", results)
	}
}

func TestUninstallResultsSupportsMsixWithoutExplicitUninstallerLocation(t *testing.T) {
	previous := installerInstallResult
	t.Cleanup(func() { installerInstallResult = previous })

	var executed []string
	installerInstallResult = func(item catalog.Item, action, _, _ string, _ bool) installer.Result {
		executed = append(executed, item.DisplayName)
		return installer.Result{ItemName: item.DisplayName, Action: action, Outcome: installer.OutcomeSucceeded}
	}

	results := UninstallResults([]string{"TestMsixInstallOnly", "TestMsixUninstall"}, testCatalogs, "", "", false)
	if want := []string{"TestMsixInstallOnly", "TestMsixUninstall"}; !reflect.DeepEqual(executed, want) {
		t.Fatalf("unexpected msix uninstall execution: got %v, want %v", executed, want)
	}
	if len(results) != 2 || results[0].Result.Outcome != installer.OutcomeSucceeded || results[1].Result.Outcome != installer.OutcomeSucceeded {
		t.Fatalf("unexpected msix uninstall results: %+v", results)
	}
}

func TestUpdateResultsExecutesActionableItems(t *testing.T) {
	previous := installerInstallResult
	t.Cleanup(func() { installerInstallResult = previous })

	var executed []string
	installerInstallResult = func(item catalog.Item, action, _, _ string, _ bool) installer.Result {
		executed = append(executed, item.DisplayName)
		return installer.Result{ItemName: item.DisplayName, Action: action, Outcome: installer.OutcomeSucceeded}
	}

	results := UpdateResults(testUpdates, testCatalogs, "", "", false)
	if !reflect.DeepEqual(executed, testUpdates) {
		t.Fatalf("unexpected update execution: got %v, want %v", executed, testUpdates)
	}
	if len(results) != len(testUpdates) {
		t.Fatalf("unexpected update results: %+v", results)
	}
}

func TestCleanUp(t *testing.T) {
	osRemove = fakeOsRemove
	t.Cleanup(func() {
		osRemove = origOsRemove
		actualRemovedFiles = nil
	})

	newTime := time.Now().Add(-24 * time.Hour)
	oldTime := time.Now().Add(-240 * time.Hour)
	emptyDir := filepath.Clean("testdata/cache/empty")
	oldFile := filepath.Clean("testdata/cache/old.msi")
	newFile := filepath.Clean("testdata/cache/new.msi")
	childFile := filepath.Clean("testdata/cache/full/file.msi")

	for path, modTime := range map[string]time.Time{
		oldFile:   oldTime,
		newFile:   newTime,
		childFile: newTime,
	} {
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(emptyDir); os.IsNotExist(err) {
		if err := os.Mkdir(emptyDir, os.ModePerm); err != nil {
			t.Fatal(err)
		}
	}

	CleanUp("testdata/")
	if want := []string{oldFile, emptyDir}; !reflect.DeepEqual(actualRemovedFiles, want) {
		t.Fatalf("unexpected cleanup removals: got %#v, want %#v", actualRemovedFiles, want)
	}
}

func fakeOsRemove(name string) error {
	actualRemovedFiles = append(actualRemovedFiles, name)
	return nil
}
