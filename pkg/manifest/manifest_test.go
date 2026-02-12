package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/config"
	yaml "go.yaml.in/yaml/v4"
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
		Name:             "example_manifest",
		Includes:         []string{"included_manifest"},
		Installs:         []string{"Chocolatey", "GoogleChrome"},
		OptionalInstalls: []string{},
		Uninstalls:       []string{"AdobeFlash"},
		Updates:          []string{"ChefClient", "CanonDrivers"},
		Catalogs:         []string{"production1", "production2"},
	}
	includedManifest = Item{
		Name:             "included_manifest",
		Includes:         []string{},
		Installs:         []string{"TestInstall1", "TestInstall2"},
		OptionalInstalls: []string{},
		Uninstalls:       []string{"TestUninstall1", "TestUninstall2"},
		Updates:          []string{"TestUpdate1", "TestUpdate2"},
		Catalogs:         []string{},
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

func TestParseServiceManifestFixture(t *testing.T) {
	serviceManifestPath := filepath.Join("testdata", "service-manifest.yaml")
	content, err := os.ReadFile(serviceManifestPath)
	if err != nil {
		t.Fatalf("failed reading fixture %s: %v", serviceManifestPath, err)
	}

	parsed := parseManifest(serviceManifestPath, content)
	if parsed.Name != "service-manifest" {
		t.Fatalf("unexpected name: %s", parsed.Name)
	}

	expectedInstalls := []string{"GoogleChrome", "7zip"}
	if !reflect.DeepEqual(expectedInstalls, parsed.Installs) {
		t.Fatalf("unexpected managed_installs, expected %#v, got %#v", expectedInstalls, parsed.Installs)
	}

	if len(parsed.Uninstalls) != 0 {
		t.Fatalf("expected no managed_uninstalls, got %#v", parsed.Uninstalls)
	}
}

func TestParseOptionalManifestFixture(t *testing.T) {
	optionalManifestPath := filepath.Join("testdata", "optional-manifest.yaml")
	content, err := os.ReadFile(optionalManifestPath)
	if err != nil {
		t.Fatalf("failed reading fixture %s: %v", optionalManifestPath, err)
	}

	parsed := parseManifest(optionalManifestPath, content)
	if parsed.Name != "optional-manifest" {
		t.Fatalf("unexpected name: %s", parsed.Name)
	}

	expectedOptionalInstalls := []string{"GoogleChrome", "VSCode"}
	if !reflect.DeepEqual(expectedOptionalInstalls, parsed.OptionalInstalls) {
		t.Fatalf("unexpected optional_installs, expected %#v, got %#v", expectedOptionalInstalls, parsed.OptionalInstalls)
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
