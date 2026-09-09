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
	"github.com/1dustindavis/gorilla/pkg/installer"
	"github.com/1dustindavis/gorilla/pkg/manifest"
	"github.com/1dustindavis/gorilla/pkg/process"
	"github.com/1dustindavis/gorilla/pkg/report"
	"github.com/1dustindavis/gorilla/pkg/status"
)

var (
	adminCheckFunc    = adminCheck
	mkdirAllFunc      = os.MkdirAll
	buildCatalogsFunc = admin.BuildCatalogs
	importItemFunc    = admin.ImportItem
)

func managedRun(cfg config.Configuration) error {
	_, err := managedRunItemResult(cfg, "", "")
	return err
}

// managedRunItemResult runs the normal managed convergence lifecycle while
// retaining structured execution evidence for one App Catalog request. Managed
// convergence uses the same result-aware process APIs for CLI, scheduled, and
// service-triggered runs.
func managedRunItemResult(cfg config.Configuration, requestedItem, requestedAction string) (installer.Result, error) {
	// Build/import modes operate on repo metadata and do not require admin.
	buildMode := cfg.BuildArg || cfg.ImportArg != ""

	// If not check-only and not build/import, we need to run adminCheck().
	if !cfg.CheckOnly && !buildMode {
		admin, err := adminCheckFunc()
		if err != nil {
			return installer.Result{}, fmt.Errorf("unable to check if running as admin: %w", err)
		}
		if !admin {
			return installer.Result{}, errors.New("gorilla requires admnisistrative access. Please run as an administrator")
		}
	}

	// If needed, create the cache directory.
	if err := mkdirAllFunc(filepath.Clean(cfg.CachePath), 0755); err != nil {
		return installer.Result{}, fmt.Errorf("unable to create cache directory: %w", err)
	}

	// Create a new logger object
	if err := gorillalog.NewLog(cfg); err != nil {
		return installer.Result{}, fmt.Errorf("unable to initialize logger: %w", err)
	}

	if cfg.BuildArg {
		gorillalog.Info("Building catalogs...")
		if err := buildCatalogsFunc(cfg.RepoPath); err != nil {
			return installer.Result{}, fmt.Errorf("error building catalogs: %w", err)
		}
		return installer.Result{}, nil
	}

	if cfg.ImportArg != "" {
		gorillalog.Info("Importing item...")
		if err := importItemFunc(cfg.RepoPath, cfg.ImportArg); err != nil {
			return installer.Result{}, fmt.Errorf("error importing item: %w", err)
		}
		return installer.Result{}, nil
	}

	// Start creating GorillaReport
	if !cfg.CheckOnly {
		report.Start()
		defer report.End()
	}

	// Set the configuration that `download` will use
	download.SetConfig(cfg)

	// Get the manifests
	gorillalog.Info("Retrieving manifest:", cfg.Manifest)
	manifests, newCatalogs, err := manifest.Get(cfg)
	if err != nil {
		return installer.Result{}, fmt.Errorf("unable to retrieve manifest: %w", err)
	}

	// If we have newCatalogs, add them to the configuration
	if newCatalogs != nil {
		cfg.Catalogs = append(cfg.Catalogs, newCatalogs...)
	}

	// Get the catalogs
	gorillalog.Info("Retrieving catalog:", cfg.Catalogs)
	catalogs, err := catalog.Get(cfg)
	if err != nil {
		return installer.Result{}, fmt.Errorf("unable to retrieve catalog: %w", err)
	}

	// Process the manifests into install type groups
	// Each managed run gets fresh registry evidence while still sharing a single
	// enumeration across all checks performed during this run.
	status.ResetRegistryCache()
	gorillalog.Info("Processing manifest...")
	installs, uninstalls, updates := process.Manifests(manifests, catalogs)

	var requested installer.Result

	// Install the full recursive dependency closure once per run. Required
	// dependencies execute before dependents; a dependent is not executed when a
	// dependency is missing, cyclic, or fails.
	gorillalog.Info("Processing managed installs...")
	installResults := process.InstallResults(installs, catalogs, cfg.URLPackages, cfg.CachePath, cfg.CheckOnly)
	if requestedAction == "InstallItem" {
		if result, ok := findManagedItemResult(installResults, requestedItem); ok {
			requested = result
		}
	}

	gorillalog.Info("Processing managed uninstalls...")
	uninstallResults := process.UninstallResults(uninstalls, catalogs, cfg.URLPackages, cfg.CachePath, cfg.CheckOnly)
	if requestedAction == "RemoveItem" {
		if result, ok := findManagedItemResult(uninstallResults, requestedItem); ok {
			requested = result
		}
	}

	// An InstallItem request may be classified as an update by manifest
	// processing, so a matching update result supersedes an earlier install
	// result when present.
	gorillalog.Info("Processing managed updates...")
	updateResults := process.UpdateResults(updates, catalogs, cfg.URLPackages, cfg.CachePath, cfg.CheckOnly)
	if requestedAction == "InstallItem" {
		if result, ok := findManagedItemResult(updateResults, requestedItem); ok {
			requested = result
		}
	}

	// Save GorillaReport to disk
	gorillalog.Info("Saving GorillaReport.json...")
	if cfg.CheckOnly {
		report.Print()
	}

	// Run CleanUp to delete old cached items and empty directories
	gorillalog.Info("Cleaning up the cache...")
	process.CleanUp(cfg.CachePath)

	gorillalog.Info("Done!")
	return requested, nil
}

func findManagedItemResult(results []process.ItemResult, itemName string) (installer.Result, bool) {
	for i := len(results) - 1; i >= 0; i-- {
		if results[i].ItemName == itemName {
			return results[i].Result, true
		}
	}
	return installer.Result{}, false
}
