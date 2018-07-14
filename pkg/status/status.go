package status

import (
	"fmt"
	"log"
	"strings"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/hashicorp/go-version"
	"golang.org/x/sys/windows/registry"
)

// Application Contiains attributes for an installed application
type Application struct {
	Key       string
	Location  string
	Name      string
	Source    string
	Uninstall string
	Version   string
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func getUninstallKeys() map[string]Application {

	// Get the Uninstall key from HKLM
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\Uninstall`, registry.READ)
	if err != nil {
		log.Fatal(err)
	}
	defer key.Close()

	// Get all the subkeys under Uninstall
	subKeys, err := key.ReadSubKeyNames(0)
	if err != nil {
		log.Fatal(err)
	}

	var installedItems map[string]Application
	installedItems = make(map[string]Application)
	// Get the details of each subkey
	for _, item := range subKeys {
		var installedItem Application
		itemKeyName := `Software\Microsoft\Windows\CurrentVersion\Uninstall\` + item
		itemKey, err := registry.OpenKey(registry.LOCAL_MACHINE, itemKeyName, registry.READ)
		if err != nil {
			log.Fatal(err)
		}
		defer key.Close()

		itemValues, err := itemKey.ReadValueNames(0)
		if stringInSlice("DisplayName", itemValues) && stringInSlice("DisplayVersion", itemValues) {
			installedItem.Key = itemKeyName
			installedItem.Name, _, err = itemKey.GetStringValue("DisplayName")
			if err != nil {
				log.Fatal(err)
			}

			installedItem.Version, _, err = itemKey.GetStringValue("DisplayVersion")
			if err != nil {
				log.Fatal(err)
			}

			installedItem.Uninstall, _, err = itemKey.GetStringValue("UninstallString")
			if err != nil {
				log.Fatal(err)
			}
			installedItems[installedItem.Name] = installedItem
		}

	}
	return installedItems
}

// UninstallReg returns the UninstallString from the registry
func UninstallReg(itemName string) string {
	// Get all installed items from the registry
	installedItems := getUninstallKeys()
	var uninstallString string

	for _, regItem := range installedItems {
		// Check if the catalog name is in the registry
		if strings.Contains(regItem.Name, itemName) {
			uninstallString = regItem.Uninstall
			break
		}
	}
	return uninstallString
}

// CheckRegistry iterates through the local registry and compiles all installed software
func CheckRegistry(catalogItem catalog.Item) (installed bool, versionMatch bool, checkErr error) {

	// Get all installed items from the registry
	installedItems := getUninstallKeys()

	// Iterate through the reg keys to compare with the catalog
	catalogVersion, err := version.NewVersion(catalogItem.Version)
	if err != nil {
		log.Fatal(err)
	}
	for _, regItem := range installedItems {
		// Check if the catalog name is in the registry
		if strings.Contains(regItem.Name, catalogItem.DisplayName) {
			installed = true

			// Check if the catalog version matches the registry
			currentVersion, err := version.NewVersion(regItem.Version)
			if err != nil {
				fmt.Println("Unable to parse current version", err)
			}
			if !currentVersion.LessThan(catalogVersion) {
				versionMatch = true
			}
			break
		}

	}

	return installed, versionMatch, checkErr

}
