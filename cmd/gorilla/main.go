package main

import (
	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/installer"
	"github.com/1dustindavis/gorilla/pkg/manifest"
)

func main() {
	// Get the configuration
	localConfig := config.Get()

	// Get the catalog
	catalog := catalog.Get(localConfig.CachePath, localConfig.Catalog, localConfig.URL)

	// Get the manifests
	manifests := manifest.Get(localConfig.CachePath, localConfig.Manifest, localConfig.URL)

	// Compile all of the installs, uninstalls, and upgrades into arrays
	var installs, uninstalls, upgrades []string
	for _, manifestItem := range manifests {
		// Installs
		for _, item := range manifestItem.Installs {
			if item != "" {
				installs = append(installs, item)
			}
		}
		// Uninstalls
		for _, item := range manifestItem.Uninstalls {
			if item != "" {
				uninstalls = append(uninstalls, item)
			}
		}
		// Upgrades
		for _, item := range manifestItem.Upgrades {
			if item != "" {
				upgrades = append(upgrades, item)
			}
		}
	}

	// Iterate through the installs array, install dependencies, and then the item itself.
	for _, item := range installs {
		// Check for dependencies and install if found
		if len(catalog[item].Dependencies) > 0 {
			for _, dependency := range catalog[item].Dependencies {
				installer.Install(catalog[dependency], localConfig.CachePath, localConfig.Verbose, localConfig.URL)
			}
		}
		// Install the item
		installer.Install(catalog[item], localConfig.CachePath, localConfig.Verbose, localConfig.URL)
	}

	// Iterate through the uninstalls array and uninstall the item.
	for _, item := range uninstalls {
		// Install the item
		installer.Uninstall(catalog[item], localConfig.CachePath, localConfig.Verbose, localConfig.URL)
	}
}
