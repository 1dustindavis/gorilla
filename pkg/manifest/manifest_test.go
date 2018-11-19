package manifest

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/config"
)

var (
	// store original data to restore after each test
	origCachePath    = config.CachePath
	origManifest     = config.Current.Manifest
	origURL          = config.Current.URL
	origDownloadFile = downloadFile
)

// TestGetManifest verifies a single manifest is processed correctly
func TestGetManifest(t *testing.T) {
	// Override the cachepath to use our test directory
	config.CachePath = "testdata/"
	defer func() { config.CachePath = origCachePath }()

	// Store the actual result of `getManifest`
	actualManifest := getManifest("example_manifest")

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

	// Override the cachepath and top manifest to use our test directory
	config.CachePath = "testdata/"
	config.Current.Manifest = "example_manifest"
	config.Current.URL = "http://example.com/"
	downloadFile = fakeDownload
	defer func() {
		config.CachePath = origCachePath
		config.Current.Manifest = origManifest
		config.Current.URL = origURL
		downloadFile = origDownloadFile
	}()

	// Store the actual slice of manifest items from `Get`
	actualManifests := Get()

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

func fakeDownload(string1 string, string2 string) error {
	fmt.Println(string1)
	fmt.Println(string2)
	return nil
}
