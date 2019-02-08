package manifest

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/config"
)

var (
	// store the current downloadFile function in order to restore later
	origDownloadFile = downloadFile
)

// TestGetManifest verifies a single manifest is processed correctly
func TestGetManifest(t *testing.T) {

	// Store the actual result of `getManifest`
	actualManifest := getManifest("testdata/", "example_manifest")

	// Define what we expect it to return
	expectedManifest := Item{
		Name:       "example_manifest",
		Includes:   []string{"included_manifest"},
		Installs:   []string{"Chocolatey", "GoogleChrome"},
		Uninstalls: []string{"AdobeFlash"},
		Updates:    []string{"ChefClient", "CanonDrivers"},
	}

	// Compare the actual result with our expectations
	structsMatch := reflect.DeepEqual(expectedManifest, actualManifest)

	if !structsMatch {
		t.Errorf("\nExpected: %#v\nActual: %#v", expectedManifest, actualManifest)
	}
}

// TestGet verifies that multiple manifests are processed correctly
func TestGet(t *testing.T) {

	// Define a Configuration struct to pass to `Get`
	cfg := config.Configuration{
		URL:       "https://example.com/",
		Manifest:  "example_manifest",
		CachePath: "testdata/",
	}

	// Override the download function, but restore it when we're done
	downloadFile = fakeDownload
	defer func() {
		downloadFile = origDownloadFile
	}()

	// Store the actual slice of manifest items that `Get` returns
	actualManifests, _ := Get(cfg)

	// Define the slice of manifest items we expect it to return
	expectedManifests := []Item{
		{
			Name:       "example_manifest",
			Includes:   []string{"included_manifest"},
			Installs:   []string{"Chocolatey", "GoogleChrome"},
			Uninstalls: []string{"AdobeFlash"},
			Updates:    []string{"ChefClient", "CanonDrivers"},
		},
		{
			Name:       "included_manifest",
			Includes:   []string(nil),
			Installs:   []string{"TestInstall1", "TestInstall2"},
			Uninstalls: []string{"TestUninstall1", "TestUninstall2"},
			Updates:    []string{"TestUpdate1", "TestUpdate2"},
		},
	}

	// Compare the actual result with our expectations
	structsMatch := reflect.DeepEqual(expectedManifests, actualManifests)

	if !structsMatch {
		t.Errorf("\nExpected: %#v\nActual: %#v", expectedManifests, actualManifests)
	}
}

// TestGetCatalogs verifies that catalogs included in a manifest get added to the config
func TestGetCatalogs(t *testing.T) {

	// Define a Configuration struct to pass to `Get`
	cfg := config.Configuration{
		URL:       "https://example.com/",
		Manifest:  "example_manifest_catalogs",
		CachePath: "testdata/",
		Catalogs:  []string{"alpha", "beta"},
	}

	// Override the download function, but restore it when we're done
	downloadFile = fakeDownload
	defer func() {
		downloadFile = origDownloadFile
	}()

	// Run Get() to process the manifests and (hopefully) append the catalogs
	_, newCatalogs := Get(cfg)

	// Define our expected catalogs
	expectedCatalogs := []string{"production1", "production2"}

	// Compare our expectations to the actual catalogs
	slicesMatch := reflect.DeepEqual(expectedCatalogs, newCatalogs)

	if !slicesMatch {
		t.Errorf("\nExpected: %#v\nActual: %#v", expectedCatalogs, newCatalogs)
	}
}

func fakeDownload(string1 string, string2 string) error {
	fmt.Println(string1)
	fmt.Println(string2)
	return nil
}
