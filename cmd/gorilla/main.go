package main

import (
	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/manifest"
	"github.com/1dustindavis/gorilla/pkg/process"
)

func main() {
	// Get the configuration
	config.Get()

	// Get the catalog
	catalog := catalog.Get()

	// Get the manifests
	manifests := manifest.Get()

	// Process the manifests into install type groups
	installs, uninstalls, updates := process.Manifests(manifests, catalog)

	// Prepare and install the install items
	process.Installs(installs, catalog)

	// Prepare and uns the install items
	process.Uninstalls(uninstalls, catalog)

	// Prepare and install the install items
	process.Updates(updates, catalog)
}
