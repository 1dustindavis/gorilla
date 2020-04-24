package manifest

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/config"
	yaml "gopkg.in/yaml.v2"
)

var (
	// store the current downloadGet function in order to restore later
	origDownloadGet = downloadGet

	// Define a Configuration struct to pass to `Get`
	cfg = config.Configuration{
		URL:            "https://example.com/",
		Manifest:       "example_manifest",
		LocalManifests: []string{"testdata/example_local_manifest.yaml"},
		Catalogs:       []string{"alpha", "beta"},
	}

	// testManifest is used to confirm the data is processed properly
	exampleManifest = Item{
		Name:       "example_manifest",
		Includes:   []string{"included_manifest"},
		Installs:   []string{"Chocolatey", "GoogleChrome"},
		Uninstalls: []string{"AdobeFlash"},
		Updates:    []string{"ChefClient", "CanonDrivers"},
		Catalogs:   []string{"production1", "production2"},
	}
	includedManifest = Item{
		Name:       "included_manifest",
		Includes:   []string{},
		Installs:   []string{"TestInstall1", "TestInstall2"},
		Uninstalls: []string{"TestUninstall1", "TestUninstall2"},
		Updates:    []string{"TestUpdate1", "TestUpdate2"},
		Catalogs:   []string{},
	}
	localManifest = Item{
		Name:       "example_local_manifest",
		Installs:   []string{"Opera"},
		Uninstalls: []string{"SomeUninstall"},
	}
)

// TestGet verifies that multiple manifests are processed correctly
func TestGet(t *testing.T) {

	// Override the download function, but restore it when we're done
	downloadGet = fakeDownload
	defer func() {
		downloadGet = origDownloadGet
	}()

	// Store the actual slice of manifest items that `Get` returns
	actualManifests, _ := Get(cfg)

	// Define the slice of manifest items we expect it to return
	expectedManifests := []Item{exampleManifest, includedManifest, localManifest}
	// 	{
	// 		Name:       "example_manifest",
	// 		Includes:   []string{"included_manifest"},
	// 		Installs:   []string{"Chocolatey", "GoogleChrome"},
	// 		Uninstalls: []string{"AdobeFlash"},
	// 		Updates:    []string{"ChefClient", "CanonDrivers"},
	// 	},
	// 	{
	// 		Name:       "included_manifest",
	// 		Includes:   []string(nil),
	// 		Installs:   []string{"TestInstall1", "TestInstall2"},
	// 		Uninstalls: []string{"TestUninstall1", "TestUninstall2"},
	// 		Updates:    []string{"TestUpdate1", "TestUpdate2"},
	// 	},
	// }

	// Compare the actual result with our expectations
	structsMatch := reflect.DeepEqual(expectedManifests, actualManifests)

	if !structsMatch {
		t.Errorf("\nExpected: %#v\nActual: %#v", expectedManifests, actualManifests)
	}
}

// TestGetCatalogs verifies that catalogs included in a manifest get added to the config
func TestGetCatalogs(t *testing.T) {

	// Override the download function, but restore it when we're done
	downloadGet = fakeDownload
	defer func() {
		downloadGet = origDownloadGet
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

// fakeDownload returns a manifest encoded as yaml based on the url passed
func fakeDownload(manifestURL string) ([]byte, error) {

	// Define a testManifest based on the url passed
	var testManifest Item
	switch manifestURL {
	case "https://example.com/manifests/example_manifest.yaml":
		fmt.Println("example!")
		testManifest = exampleManifest
	case "https://example.com/manifests/included_manifest.yaml":
		fmt.Println("included!")
		testManifest = includedManifest
	default:
		return nil, fmt.Errorf("Unexpected test url: %s", manifestURL)
	}

	// Generate yaml from the expected map
	yamlBytes, err := yaml.Marshal(testManifest)
	if err != nil {
		return nil, err
	}

	return yamlBytes, nil
}
