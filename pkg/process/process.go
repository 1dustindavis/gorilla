package process

import (
	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/installer"
	"github.com/1dustindavis/gorilla/pkg/manifest"
)

//Manifests iterates though the first manifest and any included manifests
func Manifests(manifests []manifest.Item, catalog map[string]catalog.Item) (installs, uninstalls, updates []string) {
	// Compile all of the installs, uninstalls, and updates into arrays
	for _, manifestItem := range manifests {
		// Installs
		for _, item := range manifestItem.Installs {
			if item != "" && catalog[item].InstallerItemLocation != "" {
				installs = append(installs, item)
			}
		}
		// Uninstalls
		for _, item := range manifestItem.Uninstalls {
			if item != "" {
				uninstalls = append(uninstalls, item)
			}
		}
		// Updates
		for _, item := range manifestItem.Updates {
			if item != "" {
				updates = append(updates, item)
			}
		}
	}
	return
}

// Installs prepares and then installs and array of items
func Installs(installs []string, catalog map[string]catalog.Item) {
	// Iterate through the installs array, install dependencies, and then the item itself
	for _, item := range installs {
		// Check for dependencies and install if found
		if len(catalog[item].Dependencies) > 0 {
			for _, dependency := range catalog[item].Dependencies {
				installer.Install(catalog[dependency])
			}
		}
		// Install the item
		installer.Install(catalog[item])
	}
}

// Uninstalls prepares and then installs and array of items
func Uninstalls(uninstalls []string, catalog map[string]catalog.Item) {
	// Iterate through the uninstalls array and uninstall the item
	for _, item := range uninstalls {
		// Uninstall the item
		installer.Uninstall(catalog[item])
	}
}

// Updates prepares and then installs and array of items
func Updates(updates []string, catalog map[string]catalog.Item) {
	// Iterate through the updates array and update the item **if it is already installed**
	for _, item := range updates {
		// Update the item
		installer.Update(catalog[item])
	}
}
