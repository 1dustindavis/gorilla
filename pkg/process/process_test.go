package process

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/manifest"
)

var (
	// store original data to restore after each test
	origInstall  = installerInstall
	origOsRemove = osRemove

	// Setup a test catalog
	testCatalogs = map[int]map[string]catalog.Item{1: {
		"Chocolatey": catalog.Item{
			DisplayName: "Chocolatey",
			Installer: catalog.InstallerItem{
				Type:     "msi",
				Location: "Chocolatey.msi",
			},
			Dependencies: []string{`TestUpdate1`},
		},
		"GoogleChrome": catalog.Item{
			DisplayName: "GoogleChrome",
			Installer: catalog.InstallerItem{
				Type:     "msi",
				Location: "GoogleChrome.msi",
			},
		},
		"TestInstall1": catalog.Item{
			DisplayName: "TestInstall1",
			Installer: catalog.InstallerItem{
				Type:     "msi",
				Location: "TestInstall1.msi",
			},
		},
		"TestInstall2": catalog.Item{
			DisplayName: "TestInstall2",
			Installer: catalog.InstallerItem{
				Type:     "msi",
				Location: "TestInstall2.msi",
			},
		},
		"AdobeFlash": catalog.Item{
			DisplayName: "AdobeFlash",
			Uninstaller: catalog.InstallerItem{
				Type:     "msi",
				Location: "AdobeUninst.msi",
			},
		},
		"Chef Client": catalog.Item{
			DisplayName: "Chef Client",
			Installer: catalog.InstallerItem{
				Type:     "msi",
				Location: "chef.msi",
			},
		},
		"CanonDrivers": catalog.Item{
			DisplayName: "CanonDrivers",
			Installer: catalog.InstallerItem{
				Type:     "msi",
				Location: "TestInstall1.msi",
			},
		},
		"TestUninstall1": catalog.Item{
			DisplayName: "TestUninstall1",
			Uninstaller: catalog.InstallerItem{
				Type:     "ps1",
				Location: "TestUninst2.ps1",
			},
		},
		"TestUninstall2": catalog.Item{
			DisplayName: "TestUninstall2",
			Uninstaller: catalog.InstallerItem{
				Type:     "exe",
				Location: "TestUninst2.exe",
			},
		},
		"TestUpdate1": catalog.Item{
			DisplayName: "TestUpdate1",
			Installer: catalog.InstallerItem{
				Type:     "nupkg",
				Location: "TestUpdate1.nupkg",
			},
		},
		"TestUpdate2": catalog.Item{
			DisplayName: "TestUpdate2",
			Installer: catalog.InstallerItem{
				Type:     "ps1",
				Location: "TestUpdate2.ps1",
			},
		},
	}}

	// CheckOnly flag disabled for testing
	checkOnlyMode bool = false

	// Arrays of the test items
	testInstalls   = []string{"Chocolatey", "GoogleChrome", "TestInstall1", "TestInstall2"}
	testUninstalls = []string{"AdobeFlash", "TestUninstall1", "TestUninstall2"}
	testUpdates    = []string{"Chef Client", "CanonDrivers", "TestUpdate1", "TestUpdate2"}

	// Define a variable that our fake functions can store results in
	actualInstalledItems   []string
	actualUninstalledItems []string
	actualUpdatedItems     []string
	actualRemovedFiles     []string
)

// TestManifests verifies that the installs, uninstalls, and upgrades are processed correctly
func TestManifests(t *testing.T) {

	// Setup our test manifests
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
			Includes:   []string(nil),
			Installs:   []string{"TestInstall1", "TestInstall2"},
			Uninstalls: []string{"TestUninstall1", "TestUninstall2"},
			Updates:    []string{"TestUpdate1", "TestUpdate2"},
		},
	}

	// Store the actual results of running `Manifests`
	actualInstalls, actualUninstalls, actualUpdates := Manifests(testManifests, testCatalogs)

	// Define what we expect it to return
	expectedInstalls := testInstalls
	expectedUninstalls := testUninstalls
	expectedUpdates := testUpdates

	// Compare our expectaions with the actual results
	matchInstalls := reflect.DeepEqual(expectedInstalls, actualInstalls)
	matchUninstalls := reflect.DeepEqual(expectedUninstalls, actualUninstalls)
	matchUpdates := reflect.DeepEqual(expectedUpdates, actualUpdates)

	// Fail if we dont match
	if !matchInstalls {
		t.Errorf("Manifest Installs\nExpected: %#v\nActual: %#v", expectedInstalls, actualInstalls)
	}
	if !matchUninstalls {
		t.Errorf("Manifest Uninstalls\nExpected: %#v\nActual: %#v", expectedUninstalls, actualUninstalls)
	}
	if !matchUpdates {
		t.Errorf("Manifest Updates\nExpected: %#v\nActual: %#v", expectedUpdates, actualUpdates)
	}
}

// TestInstalls tests if install items and their dependencies are processed correctly
func TestInstalls(t *testing.T) {

	// Override the install function to use our fake function
	installerInstall = fakeInstall
	defer func() { installerInstall = origInstall }()

	// Run `Installs` with test data
	Installs(testInstalls, testCatalogs, "URLPackages", "CachePath", checkOnlyMode)

	// Define what we expect to be in the list of installed items
	// This ends up being the testInstalls slice *PLUS any dependencies*
	expectedItems := append([]string{"TestUpdate1"}, testInstalls...)

	// Compare our expectaions with the actual results
	matchItems := reflect.DeepEqual(expectedItems, actualInstalledItems)

	// Fail if we dont match
	if !matchItems {
		t.Errorf("\nExpected: %#v\nActual: %#v", expectedItems, actualInstalledItems)
	}
}

// TestUninstalls tests if uninstall items are processed correctly
func TestUninstalls(t *testing.T) {

	// Override the install function to use our fake function
	installerInstall = fakeUninstall
	defer func() { installerInstall = origInstall }()

	// Run `Uninstalls` with test data
	Uninstalls(testUninstalls, testCatalogs, "URLPackages", "CachePath", checkOnlyMode)

	// Define what we expect to be in the list of uninstalled items
	expectedItems := testUninstalls

	// Compare our expectaions with the actual results
	matchItems := reflect.DeepEqual(expectedItems, actualUninstalledItems)

	// Fail if we dont match
	if !matchItems {
		t.Errorf("\nExpected: %#v\nActual: %#v", expectedItems, actualUninstalledItems)
	}
}

// TestUpdates tests if update items are processed correctly
func TestUpdates(t *testing.T) {

	// Override the install function to use our fake function
	installerInstall = fakeUpdate
	defer func() { installerInstall = origInstall }()

	// Run `Updates` with test data
	Updates(testUpdates, testCatalogs, "URLPackages", "CachePath", checkOnlyMode)

	// Define what we expect to be in the list of updated items
	expectedItems := testUpdates

	// Compare our expectaions with the actual results
	matchItems := reflect.DeepEqual(expectedItems, actualUpdatedItems)

	// Fail if we dont match
	if !matchItems {
		t.Errorf("\nExpected: %#v\nActual: %#v", expectedItems, actualUpdatedItems)
	}
}

// TestCleanUp verifies that only the correct files and directories are removed
func TestCleanUp(t *testing.T) {

	// Override the os.Remove function
	osRemove = fakeOsRemove
	defer func() {
		osRemove = origOsRemove
	}()

	// Define new and old times
	newTime := time.Now().Add(-24 * time.Hour)  // 1 day
	oldTime := time.Now().Add(-240 * time.Hour) // 10 days

	// Define the various file paths we will user
	emptyDir := filepath.Clean("testdata/cache/empty")
	oldFile := filepath.Clean("testdata/cache/old.msi")
	newFile := filepath.Clean("testdata/cache/new.msi")
	childFile := filepath.Clean("testdata/cache/full/file.msi")

	// Set the timestamps on each test file
	err := os.Chtimes(oldFile, oldTime, oldTime)
	if err != nil {
		t.Error(err)
	}
	err = os.Chtimes(newFile, newTime, newTime)
	if err != nil {
		t.Error(err)
	}
	err = os.Chtimes(childFile, newTime, newTime)
	if err != nil {
		t.Error(err)
	}

	// Create an empty directory if it doesn't already exist
	if _, err := os.Stat(emptyDir); os.IsNotExist(err) {
		// Directory does not exist
		os.Mkdir(emptyDir, os.ModePerm)
	}

	// Run `CleanUp`
	CleanUp("testdata/")

	// Define the files and directories we expect to be deleted
	expectedFiles := []string{oldFile, emptyDir}

	// Compare our expectaions with the actual results
	matchItems := reflect.DeepEqual(expectedFiles, actualRemovedFiles)

	// Fail if we dont match
	if !matchItems {
		t.Errorf("\nExpected: %#v\nActual: %#v", expectedFiles, actualRemovedFiles)
	}
}

// Mocks the actual `installer.Install` function and saves what it receives to `actualInstalledItems`
func fakeInstall(item catalog.Item, installerType string, urlPackages string, cachePath string, checkOnly bool) string {
	// Append any item we are passed to a slice for later comparison
	actualInstalledItems = append(actualInstalledItems, item.DisplayName)
	return ""
}

// Mocks the actual `installer.Install` function and saves what it receives to `actualUninstalledItems`
func fakeUninstall(item catalog.Item, installerType string, urlPackages string, cachePath string, checkOnly bool) string {
	// Append any item we are passed to a slice for later comparison
	actualUninstalledItems = append(actualUninstalledItems, item.DisplayName)
	return ""
}

// Mocks the actual `installer.Install` function and saves what it receives to `actualUpdatedItems`
func fakeUpdate(item catalog.Item, installerType string, urlPackages string, cachePath string, checkOnly bool) string {
	// Append any item we are passed to a slice for later comparison
	actualUpdatedItems = append(actualUpdatedItems, item.DisplayName)
	return ""
}

// Mock `os.Remove` so we dont delete files during testing
func fakeOsRemove(name string) error {
	actualRemovedFiles = append(actualRemovedFiles, name)
	return nil
}
