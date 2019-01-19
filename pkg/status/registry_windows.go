// +build windows

package status

import (
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
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
