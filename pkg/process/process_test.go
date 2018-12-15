package process

import (
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/manifest"
)

var (
	// store original data to restore after each test
	origInstall     = installerInstall
	origUninstall   = installerUninstall
	origUpdate      = installerUpdate
	origAppDataPath = config.Current.AppDataPath
	origOsRemove    = osRemove

	// Setup a test catalog
	testCatalog = map[string]catalog.Item{
		"Chocolatey":     catalog.Item{DisplayName: "Chocolatey", InstallerItemLocation: "Chocolatey.msi", Dependencies: []string{`TestUpdate1`}},
		"GoogleChrome":   catalog.Item{DisplayName: "GoogleChrome", InstallerItemLocation: "GoogleChrome.msi"},
		"TestInstall1":   catalog.Item{DisplayName: "TestInstall1", InstallerItemLocation: "TestInstall1.msi"},
		"TestInstall2":   catalog.Item{DisplayName: "TestInstall2", InstallerItemLocation: "TestInstall2.msi"},
		"AdobeFlash":     catalog.Item{DisplayName: "AdobeFlash"},
		"Chef Client":    catalog.Item{DisplayName: "Chef Client"},
		"CanonDrivers":   catalog.Item{DisplayName: "CanonDrivers"},
		"TestUninstall1": catalog.Item{DisplayName: "TestUninstall1"},
		"TestUninstall2": catalog.Item{DisplayName: "TestUninstall2"},
		"TestUpdate1":    catalog.Item{DisplayName: "TestUpdate1"},
		"TestUpdate2":    catalog.Item{DisplayName: "TestUpdate2"},
	}

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
	actualInstalls, actualUninstalls, actualUpdates := Manifests(testManifests, testCatalog)

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
		t.Errorf("\nExpected: %#v\nActual: %#v", expectedInstalls, actualInstalls)
	}
	if !matchUninstalls {
		t.Errorf("\nExpected: %#v\nActual: %#v", expectedUninstalls, actualUninstalls)
	}
	if !matchUpdates {
		t.Errorf("\nExpected: %#v\nActual: %#v", expectedUpdates, actualUpdates)
	}
}

// TestInstalls tests if install items and their dependencies are processed correctly
func TestInstalls(t *testing.T) {

	// Override the install function to use our fake function
	installerInstall = fakeInstall
	defer func() { installerInstall = origInstall }()

	// Run `Installs` with test data
	Installs(testInstalls, testCatalog)

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

	// Override the uninstall function to use our fake function
	installerUninstall = fakeUninstall
	defer func() { installerUninstall = origUninstall }()

	// Run `Uninstalls` with test data
	Uninstalls(testUninstalls, testCatalog)

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

	// Override the update function to use our fake function
	installerUpdate = fakeUpdate
	defer func() { installerUpdate = origUpdate }()

	// Run `Updates` with test data
	Updates(testUpdates, testCatalog)

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

	// Override the cachepath and the os.Remove function
	config.Current.AppDataPath = "testdata/"
	osRemove = fakeOsRemove
	defer func() {
		osRemove = origOsRemove
		config.Current.AppDataPath = origAppDataPath
	}()

	// Define new and old times
	newTime := time.Now().Add(-24 * time.Hour)  // 1 day
	oldTime := time.Now().Add(-240 * time.Hour) // 10 days

	// Set the timestamps on each test file
	err := os.Chtimes("testdata/cache/old.msi", oldTime, oldTime)
	if err != nil {
		t.Error(err)
	}
	err = os.Chtimes("testdata/cache/new.msi", newTime, newTime)
	if err != nil {
		t.Error(err)
	}
	err = os.Chtimes("testdata/cache/full/file.msi", newTime, newTime)
	if err != nil {
		t.Error(err)
	}

	// Create an empty directory if it doesn't already exist
	if _, err := os.Stat("testdata/cache/empty"); os.IsNotExist(err) {
		// Directory does not exist
		os.Mkdir("testdata/cache/empty", os.ModePerm)
	}

	// Run `CleanUp`
	CleanUp()

	// Define the files and directories we expect to be deleted
	expectedFiles := []string{"testdata/cache/old.msi", "testdata/cache/empty"}

	// Compare our expectaions with the actual results
	matchItems := reflect.DeepEqual(expectedFiles, actualRemovedFiles)

	// Fail if we dont match
	if !matchItems {
		t.Errorf("\nExpected: %#v\nActual: %#v", expectedFiles, actualRemovedFiles)
	}
}

// Mocks the actual `installer.Install` function
func fakeInstall(item catalog.Item) string {
	// Append any item we are passed to a slice for later comparison
	actualInstalledItems = append(actualInstalledItems, item.DisplayName)
	return ""
}

// Mocks the actual `installer.Uninstall` function
func fakeUninstall(item catalog.Item) string {
	// Append any item we are passed to a slice for later comparison
	actualUninstalledItems = append(actualUninstalledItems, item.DisplayName)
	return ""
}

// Mocks the actual `installer.Update` function
func fakeUpdate(item catalog.Item) string {
	// Append any item we are passed to a slice for later comparison
	actualUpdatedItems = append(actualUpdatedItems, item.DisplayName)
	return ""
}

// Mock `os.Remove` so we dont delete files during testing
func fakeOsRemove(name string) error {
	actualRemovedFiles = append(actualRemovedFiles, name)
	return nil
}
