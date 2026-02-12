package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/1dustindavis/gorilla/pkg/admin"
	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/download"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	"github.com/1dustindavis/gorilla/pkg/manifest"
	"github.com/1dustindavis/gorilla/pkg/process"
	"github.com/1dustindavis/gorilla/pkg/report"
)

var (
	adminCheckFunc         = adminCheck
	mkdirAllFunc           = os.MkdirAll
	buildCatalogsFunc      = admin.BuildCatalogs
	importItemFunc         = admin.ImportItem
	managedRunFunc         = managedRun
	runServiceFunc         = runService
	sendServiceCommandFunc = sendServiceCommand
	runServiceActionFunc   = runServiceAction
)

func main() {
	cfg := config.Get()
	if err := execute(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func execute(cfg config.Configuration) error {
	if cfg.ServiceInstall {
		return runServiceActionFunc(cfg, "install")
	}

	if cfg.ServiceRemove {
		return runServiceActionFunc(cfg, "remove")
	}

	if cfg.ServiceStart {
		return runServiceActionFunc(cfg, "start")
	}

	if cfg.ServiceStop {
		return runServiceActionFunc(cfg, "stop")
	}

	if cfg.ServiceCommand != "" {
		return sendServiceCommandFunc(cfg, cfg.ServiceCommand)
	}

	if cfg.ServiceMode {
		return runServiceFunc(cfg)
	}

	return managedRunFunc(cfg)
}

func managedRun(cfg config.Configuration) error {
	// Build/import modes operate on repo metadata and do not require admin.
	buildMode := cfg.BuildArg || cfg.ImportArg != ""

	// If not check-only and not build/import, we need to run adminCheck().
	if !cfg.CheckOnly && !buildMode {
		admin, err := adminCheckFunc()
		if err != nil {
			return fmt.Errorf("unable to check if running as admin: %w", err)
		}
		if !admin {
			return errors.New("gorilla requires admnisistrative access. Please run as an administrator")
		}
	}

	// If needed, create the cache directory.
	if err := mkdirAllFunc(filepath.Clean(cfg.CachePath), 0755); err != nil {
		return fmt.Errorf("unable to create cache directory: %w", err)
	}

	// Create a new logger object
	gorillalog.NewLog(cfg)

	if cfg.BuildArg {
		gorillalog.Info("Building catalogs...")
		if err := buildCatalogsFunc(cfg.RepoPath); err != nil {
			return fmt.Errorf("error building catalogs: %w", err)
		}
		return nil
	}

	if cfg.ImportArg != "" {
		gorillalog.Info("Importing item...")
		if err := importItemFunc(cfg.RepoPath, cfg.ImportArg); err != nil {
			return fmt.Errorf("error importing item: %w", err)
		}
		return nil
	}

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
	return nil
}
