package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/download"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	"github.com/1dustindavis/gorilla/pkg/manifest"
	"github.com/1dustindavis/gorilla/pkg/process"
	"github.com/1dustindavis/gorilla/pkg/report"
)

func main() {

	// Get our configuration
	cfg := config.Get()
	var err error
	// if --checkonly is NOT passed, we need to run adminCheck()
	if !cfg.CheckOnly {
		admin, err := adminCheck()
		if err != nil {
			fmt.Println("Unable to check if running as admin, got: %w", err)
			os.Exit(1)
		}
		if !admin {
			fmt.Println("Gorilla requires admnisistrative access. Please run as an administrator.")
			os.Exit(1)
		}
	}

	// If needed, create the cache directory
	err = os.MkdirAll(filepath.Clean(cfg.CachePath), 0755)
	if err != nil {
		fmt.Println("Unable to create cache directory: ", err)
		os.Exit(1)
	}

	// Create a new logger object
	gorillalog.NewLog(cfg)

	// Start creating GorillaReport
	if !cfg.CheckOnly {
		report.Start()
	}

	// Set the configuration that `download` will use
	download.SetConfig(cfg)

	// Get the manifests
	gorillalog.Info("Retrieving manifest:", cfg.Manifest)
	manifests, newCatalogs := manifest.Get(cfg)

	// If we have newCatalogs, add them to the configuration
	if newCatalogs != nil {
		cfg.Catalogs = append(cfg.Catalogs, newCatalogs...)
	}

	// Get the catalogs
	gorillalog.Info("Retrieving catalog:", cfg.Catalogs)
	catalogs := catalog.Get(cfg)

	// Process the manifests into install type groups
	gorillalog.Info("Processing manifest...")
	installs, uninstalls, updates := process.Manifests(manifests, catalogs)

	// Prepare and install
	gorillalog.Info("Processing managed installs...")
	process.Installs(installs, catalogs, cfg.URLPackages, cfg.CachePath, cfg.CheckOnly)

	// Prepare and uninstall
	gorillalog.Info("Processing managed uninstalls...")
	process.Uninstalls(uninstalls, catalogs, cfg.URLPackages, cfg.CachePath, cfg.CheckOnly)

	// Prepare and update
	gorillalog.Info("Processing managed updates...")
	process.Updates(updates, catalogs, cfg.URLPackages, cfg.CachePath, cfg.CheckOnly)

	// Save GorillaReport to disk
	gorillalog.Info("Saving GorillaReport.json...")
	if !cfg.CheckOnly {
		report.End()
	} else {
		report.Print()
	}

	// Run CleanUp to delete old cached items and empty directories
	gorillalog.Info("Cleaning up the cache...")
	process.CleanUp(cfg.CachePath)

	gorillalog.Info("Done!")
}
