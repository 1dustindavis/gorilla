// +build windows

package status

import (
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	registry "golang.org/x/sys/windows/registry"
)

// checkValues returns true if all of our desired values exist
func checkValues(values []string) (valuesExist bool) {
	var nameExists bool
	var versionExists bool
	var uninstallExists bool

	for _, value := range values {
		if value == "DisplayName" {
			nameExists = true
		}
		if value == "DisplayVersion" {
			versionExists = true
		}
		if value == "UninstallString" {
			uninstallExists = true
		}
	}

	return nameExists && versionExists && uninstallExists
}

func getUninstallKeys() (installedItems map[string]RegistryApplication, checkErr error) {
	// Initialize the map we will add any values to
	installedItems = make(map[string]RegistryApplication)

	// Both Uninstall paths (64 & 32 bits apps)
	regPaths := []string{`Software\Microsoft\Windows\CurrentVersion\Uninstall`,
		`Software\Wow6432Node\Microsoft\Windows\CurrentVersion\Uninstall`}

	for _, regPath := range regPaths {

		// Get the Uninstall key from HKLM
		key, checkErr := registry.OpenKey(registry.LOCAL_MACHINE, regPath, registry.READ)
		if checkErr != nil {
			gorillalog.Warn("Unable to read registry key:", checkErr)
			return installedItems, checkErr
		}
		defer key.Close()

		// Get all the subkeys under Uninstall
		subKeys, checkErr := key.ReadSubKeyNames(0)
		if checkErr != nil {
			gorillalog.Warn("Unable to read registry sub keys:", checkErr)
			return installedItems, checkErr
		}

		// Get the details of each subkey and add them to a map of `RegistryApplication`
		for _, item := range subKeys {

			//  installedItem is the struct we will store each application in
			var installedItem RegistryApplication
			itemKeyName := regPath + `\` + item
			itemKey, checkErr := registry.OpenKey(registry.LOCAL_MACHINE, itemKeyName, registry.READ)
			if checkErr != nil {
				gorillalog.Warn("Unable to read registry key:", checkErr)
				return installedItems, checkErr
			}
			defer itemKey.Close()

			// Put the names of all the values in a slice
			itemValues, checkErr := itemKey.ReadValueNames(0)
			if checkErr != nil {
				gorillalog.Warn("Unable to read registry value names:", checkErr)
				return installedItems, checkErr
			}

			// If checkValues() returns true, add the values to our struct
			if checkValues(itemValues) {
				installedItem.Key = itemKeyName
				installedItem.Name, _, checkErr = itemKey.GetStringValue("DisplayName")
				if checkErr != nil {
					gorillalog.Warn("Unable to read DisplayName", checkErr)
					return installedItems, checkErr
				}

				installedItem.Version, _, checkErr = itemKey.GetStringValue("DisplayVersion")
				if checkErr != nil {
					gorillalog.Warn("Unable to read DisplayVersion", checkErr)
					return installedItems, checkErr
				}

				installedItem.Uninstall, _, checkErr = itemKey.GetStringValue("UninstallString")
				if checkErr != nil {
					gorillalog.Warn("Unable to read UninstallString", checkErr)
					return installedItems, checkErr
				}
				installedItems[installedItem.Name] = installedItem
			}
		}
	}
	return installedItems, checkErr
}
