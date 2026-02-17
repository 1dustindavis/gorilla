package catalog

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/config"
	"go.yaml.in/yaml/v4"
)

var expected = make(map[string]Item)

func fakeDownload(string string) ([]byte, error) {
	fmt.Println(string)

	// Generate yaml from the expected map
	yamlBytes, err := yaml.Marshal(expected)
	if err != nil {
		return nil, err
	}

	return yamlBytes, nil
}

func fakeDownloadByURL(payloads map[string][]byte, failures map[string]error) func(string) ([]byte, error) {
	return func(url string) ([]byte, error) {
		if err, ok := failures[url]; ok {
			return nil, err
		}
		if body, ok := payloads[url]; ok {
			return body, nil
		}
		return nil, fmt.Errorf("unexpected URL in test: %s", url)
	}
}

// TestGet verifies that a valid catlog is parsed correctly and returns the expected map
func TestGet(t *testing.T) {
	expected = make(map[string]Item)
	// Set what we expect Get() to return
	expected[`ChefClient`] = Item{
		Dependencies: []string{`ruby`},
		DisplayName:  "Chef Client",
		Check: InstallCheck{
			File: []FileCheck{{Path: `C:\opscode\chef\bin\chef-client.bat`}, {Path: `C:\test\path\check\file.exe`, Hash: `abc1234567890def`, Version: `1.2.3.0`}},
			Script: `$latest = "14.3.37"
$current = C:\opscode\chef\bin\chef-client.bat --version
$current = $current.Split(" ")[1]
$upToDate = [System.Version]$current -ge [System.Version]$latest
If ($upToDate) {
  exit 1
} Else {
  exit 0
}
`},
		Installer: InstallerItem{
			Arguments: []string{`/L=1033`, `/S`},
			Hash:      `f5ef8c31898592824751ec2252fe317c0f667db25ac40452710c8ccf35a1b28d`,
			Location:  `packages/chef-client/chef-client-14.3.37-1-x64.msi`,
		},
		Uninstaller:  InstallerItem{Type: `msi`, Arguments: []string{`/S`}},
		Version:      `68.0.3440.106`,
		BlockingApps: []string{"test"},
	}

	// Define a Configuration struct to pass to `Get`
	cfg := config.Configuration{
		URL:       "https://example.com/",
		Manifest:  "example_manifest",
		CachePath: "testdata/",
		Catalogs:  []string{"test_catalog"},
	}

	// Override the downloadFile function with our fake function
	origDownload := downloadGet
	defer func() { downloadGet = origDownload }()
	downloadGet = fakeDownload

	// Run `Get`
	testCatalog, err := Get(cfg)
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	mapsMatch := reflect.DeepEqual(expected, testCatalog[1])

	if !mapsMatch {
		t.Errorf("\n\nExpected:\n\n%#v\n\nReceived:\n\n %#v", expected, testCatalog[1])
	}
}

func TestGetReturnsErrorForMissingCatalog(t *testing.T) {
	baseCatalog := map[string]Item{
		"Chrome": {
			DisplayName: "Chrome",
			Installer: InstallerItem{
				Type:     "nupkg",
				Location: "packages/chrome/chrome.nupkg",
				Hash:     "abc",
			},
		},
	}
	baseYAML, err := yaml.Marshal(baseCatalog)
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.Configuration{
		URL:      "https://example.com/",
		Catalogs: []string{"base", "missing"},
	}

	origDownload := downloadGet
	defer func() { downloadGet = origDownload }()
	downloadGet = fakeDownloadByURL(
		map[string][]byte{
			"https://example.com/catalogs/base.yaml": baseYAML,
		},
		map[string]error{
			"https://example.com/catalogs/missing.yaml": errors.New("404"),
		},
	)

	_, err = Get(cfg)
	if err == nil {
		t.Fatalf("expected Get() to fail for missing catalog")
	}
}

func TestGetReturnsErrorForInvalidYAML(t *testing.T) {
	baseCatalog := map[string]Item{
		"ChefClient": {
			DisplayName: "Chef Client",
			Installer: InstallerItem{
				Type:     "msi",
				Location: "packages/chef/chef.msi",
				Hash:     "abc",
			},
		},
	}
	baseYAML, err := yaml.Marshal(baseCatalog)
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.Configuration{
		URL:      "https://example.com/",
		Catalogs: []string{"valid", "broken"},
	}

	origDownload := downloadGet
	defer func() { downloadGet = origDownload }()
	downloadGet = fakeDownloadByURL(
		map[string][]byte{
			"https://example.com/catalogs/valid.yaml":  baseYAML,
			"https://example.com/catalogs/broken.yaml": []byte(":\n- not valid yaml"),
		},
		nil,
	)

	_, err = Get(cfg)
	if err == nil {
		t.Fatalf("expected Get() to fail for invalid catalog YAML")
	}
}

func TestGetNoCatalogsReturnsError(t *testing.T) {
	cfg := config.Configuration{
		URL:      "https://example.com/",
		Catalogs: []string{},
	}

	_, err := Get(cfg)
	if err == nil {
		t.Fatalf("expected error when no catalogs are configured")
	}
}
