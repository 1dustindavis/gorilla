package main

import (
	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	"github.com/1dustindavis/gorilla/pkg/manifest"
	"github.com/1dustindavis/gorilla/pkg/process"
	"github.com/pkg/profile"
)

func main() {

	// CPU profiling by default
	defer profile.Start().Stop()

	// Create a new logger object
	gorillalog.NewLog()

	// Get our configuration
	config.Get()

	// Get the catalog
	gorillalog.Info("Retrieving catalog:", config.Catalog)
	catalog := catalog.Get()

	// Get the manifests
	gorillalog.Info("Retrieving manifest:", config.Manifest)
	manifests := manifest.Get()

	// Process the manifests into install type groups
	gorillalog.Info("Processing manifest...")
	installs, uninstalls, updates := process.Manifests(manifests, catalog)

	// Prepare and install
	gorillalog.Info("Processing managed installs...")
	process.Installs(installs, catalog)

	// Prepare and uninstall
	gorillalog.Info("Processing managed uninstalls...")
	process.Uninstalls(uninstalls, catalog)

	// Prepare and update
	gorillalog.Info("Processing managed updates...")
	process.Updates(updates, catalog)

	gorillalog.Info("Done!")
}
