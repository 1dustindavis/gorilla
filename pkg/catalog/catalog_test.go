package catalog

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/config"
)

func fakeDownload(string1 string, string2 string) error {
	fmt.Println(string1)
	fmt.Println(string2)
	return nil
}

// TestGet verifies that a valid catlog is parsed correctly and returns the expected map
func TestGet(t *testing.T) {
	// Set what we expect Get() to return
	var expected = make(map[string]Item)
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
		Uninstaller: InstallerItem{Type: `msi`},
		Version:     `68.0.3440.106`,
	}

	// Define a Configuration struct to pass to `Get`
	cfg := config.Configuration{
		URL:       "https://example.com/",
		Manifest:  "example_manifest",
		CachePath: "testdata/",
		Catalogs:  []string{"test_catalog"},
	}

	// Override the downloadFile function with our fake function
	downloadFile = fakeDownload

	// Run `Get`
	testCatalog := Get(cfg)

	mapsMatch := reflect.DeepEqual(expected, testCatalog[1])

	if !mapsMatch {
		t.Errorf("\n\nExpected:\n\n%#v\n\nReceived:\n\n %#v", expected, testCatalog[1])
	}
}
