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

var expected map[string]Item

// TestGet verifies that a valid catlog is parsed correctly and returns the expected map
func TestGet(t *testing.T) {
	// Set what we expect Get() to return
	expected = make(map[string]Item)
	expected[`ChefClient`] = Item{
		Dependencies:     []string{`ruby`},
		DisplayName:      "Chef Client",
		InstallCheckPath: `C:\opscode\chef\bin\chef-client.bat`,
		InstallCheckScript: `$latest = "14.3.37"
$current = C:\opscode\chef\bin\chef-client.bat --version
$current = $current.Split(" ")[1]
$upToDate = [System.Version]$current -ge [System.Version]$latest
If ($upToDate) {
  exit 1
} Else {
  exit 0
}
`,
		InstallerItemArguments: []string{`/L=1033`, `/S`},
		InstallerItemHash:      `f5ef8c31898592824751ec2252fe317c0f667db25ac40452710c8ccf35a1b28d`,
		InstallerItemLocation:  `packages/chef-client/chef-client-14.3.37-1-x64.msi`,
		UninstallMethod:        `msi`,
		Version:                `68.0.3440.106`,
	}

	config.Current.URL = "http://example.com/"
	config.Current.Catalog = "test_catalog"
	config.CachePath = "testdata/"
	downloadFile = fakeDownload
	testCatalog := Get()

	mapsMatch := reflect.DeepEqual(expected, testCatalog)

	if !mapsMatch {
		t.Errorf("\n\nExpected:\n\n%#v\n\nReceived:\n\n %#v", expected, testCatalog)
	}
}
