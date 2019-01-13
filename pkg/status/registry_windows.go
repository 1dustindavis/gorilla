// +build windows

package status

import (
	"strings"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	version "github.com/hashicorp/go-version"
	registry "golang.org/x/sys/windows/registry"
)

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func getUninstallKeys() map[string]Application {

	// Recover when the registry lookup fails
	defer func() {
		if r := recover(); r != nil {
			gorillalog.Warn("Recovered from error while accessing the registry: ", r)
		}
	}()

	// Get the Uninstall key from HKLM
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\Uninstall`, registry.READ)
	if err != nil {
		gorillalog.Error("Unable to read registry key", err)
	}
	defer key.Close()

	// Get all the subkeys under Uninstall
	subKeys, err := key.ReadSubKeyNames(0)
	if err != nil {
		gorillalog.Error("Unable to read registry sub key:", err)
	}

	installedItems := make(map[string]Application)
	// Get the details of each subkey
	for _, item := range subKeys {
		var installedItem Application
		itemKeyName := `Software\Microsoft\Windows\CurrentVersion\Uninstall\` + item
		itemKey, err := registry.OpenKey(registry.LOCAL_MACHINE, itemKeyName, registry.READ)
		if err != nil {
			gorillalog.Error("Unable to read registry key", err)
		}
		defer key.Close()

		itemValues, err := itemKey.ReadValueNames(0)
		if stringInSlice("DisplayName", itemValues) && stringInSlice("DisplayVersion", itemValues) {
			installedItem.Key = itemKeyName
			installedItem.Name, _, err = itemKey.GetStringValue("DisplayName")
			if err != nil {
				gorillalog.Error("Unable to read DisplayName", err)
			}

			installedItem.Version, _, err = itemKey.GetStringValue("DisplayVersion")
			if err != nil {
				gorillalog.Error("Unable to read DisplayVersion", err)
			}

			installedItem.Uninstall, _, err = itemKey.GetStringValue("UninstallString")
			if err != nil {
				gorillalog.Error("Unable to read UninstallString", err)
			}
			installedItems[installedItem.Name] = installedItem
		}

	}
	return installedItems
}

// checkRegistry iterates through the local registry and compiles all installed software
func checkRegistry(catalogItem catalog.Item, installType string) (actionNeeded bool, checkErr error) {
	// Iterate through the reg keys to compare with the catalog
	catalogVersion, err := version.NewVersion(catalogItem.Version)
	if err != nil {
		gorillalog.Warn("Unable to parse new version: ", catalogItem.DisplayName, err)
	}

	var installed bool
	var versionMatch bool
	for _, regItem := range RegistryItems {
		// Check if the catalog name is in the registry
		if strings.Contains(regItem.Name, catalogItem.DisplayName) {
			installed = true

			// Check if the catalog version matches the registry
			currentVersion, err := version.NewVersion(regItem.Version)
			if err != nil {
				gorillalog.Warn("Unable to parse current version", err)
			}
			if !currentVersion.LessThan(catalogVersion) {
				versionMatch = true
			}
			break
		}

	}

	// If we don't have version information, we can't compare
	if catalogItem.Version == "" {
		versionMatch = true
	}

	if installType == "update" && !installed {
		actionNeeded = false
	} else if installType == "uninstall" && installed {
		actionNeeded = true
	} else if installed && versionMatch {
		actionNeeded = false
	} else {
		actionNeeded = true
	}

	return actionNeeded, checkErr

}
