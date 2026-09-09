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
// retaining the structured result for one App Catalog request. It deliberately
// preserves legacy managed-run dependency semantics; it does not activate the
// recursive item-scoped execution path.
func managedRunItemResult(cfg config.Configuration, requestedItem, requestedAction string) (process.ItemResult, error) {
	// Build/import modes operate on repo metadata and do not require admin.
	buildMode := cfg.BuildArg || cfg.ImportArg != ""

	// If not check-only and not build/import, we need to run adminCheck().
	if !cfg.CheckOnly && !buildMode {
		admin, err := adminCheckFunc()
		if err != nil {
			return process.ItemResult{}, fmt.Errorf("unable to check if running as admin: %w", err)
		}
		if !admin {
			return process.ItemResult{}, errors.New("gorilla requires admnisistrative access. Please run as an administrator")
		}
	}

	// If needed, create the cache directory.
	if err := mkdirAllFunc(filepath.Clean(cfg.CachePath), 0755); err != nil {
		return process.ItemResult{}, fmt.Errorf("unable to create cache directory: %w", err)
	}

	// Create a new logger object
	if err := gorillalog.NewLog(cfg); err != nil {
		return process.ItemResult{}, fmt.Errorf("unable to initialize logger: %w", err)
	}

	if cfg.BuildArg {
		gorillalog.Info("Building catalogs...")
		if err := buildCatalogsFunc(cfg.RepoPath); err != nil {
			return process.ItemResult{}, fmt.Errorf("error building catalogs: %w", err)
		}
		return process.ItemResult{}, nil
	}

	if cfg.ImportArg != "" {
		gorillalog.Info("Importing item...")
		if err := importItemFunc(cfg.RepoPath, cfg.ImportArg); err != nil {
			return process.ItemResult{}, fmt.Errorf("error importing item: %w", err)
		}
		return process.ItemResult{}, nil
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
		return process.ItemResult{}, fmt.Errorf("unable to retrieve manifest: %w", err)
	}

	// If we have newCatalogs, add them to the configuration
	if newCatalogs != nil {
		cfg.Catalogs = append(cfg.Catalogs, newCatalogs...)
	}

	// Get the catalogs
	gorillalog.Info("Retrieving catalog:", cfg.Catalogs)
	catalogs, err := catalog.Get(cfg)
	if err != nil {
		return process.ItemResult{}, fmt.Errorf("unable to retrieve catalog: %w", err)
	}

	// Process the manifests into install type groups
	// Each managed run gets fresh registry evidence while still sharing a single
	// enumeration across all checks performed during this run.
	status.ResetRegistryCache()
	gorillalog.Info("Processing manifest...")
	installs, uninstalls, updates := process.Manifests(manifests, catalogs)

	var requested process.ItemResult
	captureResults := requestedItem != ""

	// Prepare and install
	gorillalog.Info("Processing managed installs...")
	if captureResults {
		results := process.ManagedInstallResults(installs, catalogs, cfg.URLPackages, cfg.CachePath, cfg.CheckOnly)
		if requestedAction == "InstallItem" {
			requested = findManagedItemResult(results, requestedItem)
		}
	} else {
		process.Installs(installs, catalogs, cfg.URLPackages, cfg.CachePath, cfg.CheckOnly)
	}

	// Prepare and uninstall
	gorillalog.Info("Processing managed uninstalls...")
	if captureResults {
		results := process.UninstallResults(uninstalls, catalogs, cfg.URLPackages, cfg.CachePath, cfg.CheckOnly)
		if requestedAction == "RemoveItem" {
			requested = findManagedItemResult(results, requestedItem)
		}
	} else {
		process.Uninstalls(uninstalls, catalogs, cfg.URLPackages, cfg.CachePath, cfg.CheckOnly)
	}

	// Prepare and update
	gorillalog.Info("Processing managed updates...")
	if captureResults {
		_ = process.UpdateResults(updates, catalogs, cfg.URLPackages, cfg.CachePath, cfg.CheckOnly)
	} else {
		process.Updates(updates, catalogs, cfg.URLPackages, cfg.CachePath, cfg.CheckOnly)
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
	if captureResults && requested.ItemName == "" {
		requested = process.ItemResult{
			ItemName: requestedItem,
			Result: installer.Result{
				ItemName:  requestedItem,
				Action:    requestedAction,
				Outcome:   installer.OutcomeFailed,
				ErrorCode: "requested_item_not_processed",
				Message:   "Requested item was not processed by the managed run",
			},
		}
	}
	return requested, nil
}

func findManagedItemResult(results []process.ItemResult, itemName string) process.ItemResult {
	for i := len(results) - 1; i >= 0; i-- {
		if results[i].ItemName == itemName {
			return results[i]
		}
	}
	return process.ItemResult{}
}
