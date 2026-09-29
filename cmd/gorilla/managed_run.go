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
	adminCheckFunc               = adminCheck
	mkdirAllFunc                 = os.MkdirAll
	buildCatalogsFunc            = admin.BuildCatalogs
	importItemFunc               = admin.ImportItem
	managedResultWarnFunc        = gorillalog.Warn
	manifestGetFunc              = manifest.Get
	catalogGetFunc               = catalog.Get
	processManifestsFunc         = process.Manifests
	processInstallResultsFunc    = process.InstallResults
	processUninstallResultsFunc  = process.UninstallResults
	processUpdateResultsFunc     = process.UpdateResults
	processCleanUpFunc           = process.CleanUp
	statusResetRegistryCacheFunc = status.ResetRegistryCache
)

type managedExecutionContext struct {
	cfg       config.Configuration
	catalogs  map[int]map[string]catalog.Item
	manifests []manifest.Item
}

func prepareManagedEnvironment(cfg config.Configuration, requireAdmin bool) error {
	if requireAdmin && !cfg.CheckOnly {
		isAdmin, err := adminCheckFunc()
		if err != nil {
			return fmt.Errorf("unable to check if running as admin: %w", err)
		}
		if !isAdmin {
			return errors.New("gorilla requires admnisistrative access. Please run as an administrator")
		}
	}

	if err := mkdirAllFunc(filepath.Clean(cfg.CachePath), 0755); err != nil {
		return fmt.Errorf("unable to create cache directory: %w", err)
	}

	if err := gorillalog.NewLog(cfg); err != nil {
		return fmt.Errorf("unable to initialize logger: %w", err)
	}
	return nil
}

func prepareManagedExecution(cfg config.Configuration) (managedExecutionContext, error) {
	download.SetConfig(cfg)

	gorillalog.Info("Retrieving manifest:", cfg.Manifest)
	manifests, newCatalogs, err := manifestGetFunc(cfg)
	if err != nil {
		return managedExecutionContext{}, fmt.Errorf("unable to retrieve manifest: %w", err)
	}

	if newCatalogs != nil {
		cfg.Catalogs = append(cfg.Catalogs, newCatalogs...)
	}

	gorillalog.Info("Retrieving catalog:", cfg.Catalogs)
	catalogs, err := catalogGetFunc(cfg)
	if err != nil {
		return managedExecutionContext{}, fmt.Errorf("unable to retrieve catalog: %w", err)
	}

	// Each managed execution gets fresh registry evidence while still sharing a
	// single enumeration across all checks performed during that execution.
	statusResetRegistryCacheFunc()

	return managedExecutionContext{
		cfg:       cfg,
		catalogs:  catalogs,
		manifests: manifests,
	}, nil
}

func managedRun(cfg config.Configuration) error {
	// Build/import modes operate on repo metadata and do not require admin.
	buildMode := cfg.BuildArg || cfg.ImportArg != ""
	if buildMode {
		if err := prepareManagedEnvironment(cfg, false); err != nil {
			return err
		}

		if cfg.BuildArg {
			gorillalog.Info("Building catalogs...")
			if err := buildCatalogsFunc(cfg.RepoPath); err != nil {
				return fmt.Errorf("error building catalogs: %w", err)
			}
			return nil
		}

		gorillalog.Info("Importing item...")
		if err := importItemFunc(cfg.RepoPath, cfg.ImportArg); err != nil {
			return fmt.Errorf("error importing item: %w", err)
		}
		return nil
	}

	if err := prepareManagedEnvironment(cfg, true); err != nil {
		return err
	}

	if !cfg.CheckOnly {
		report.Start()
		defer report.End()
	}

	ctx, err := prepareManagedExecution(cfg)
	if err != nil {
		return err
	}

	gorillalog.Info("Processing manifest...")
	installs, uninstalls, updates := processManifestsFunc(ctx.manifests, ctx.catalogs)

	gorillalog.Info("Processing managed installs...")
	installResults := processInstallResultsFunc(installs, ctx.catalogs, ctx.cfg.URLPackages, ctx.cfg.CachePath, ctx.cfg.CheckOnly)
	logManagedResultFailures("install", installResults)

	gorillalog.Info("Processing managed uninstalls...")
	uninstallResults := processUninstallResultsFunc(uninstalls, ctx.catalogs, ctx.cfg.URLPackages, ctx.cfg.CachePath, ctx.cfg.CheckOnly)
	logManagedResultFailures("uninstall", uninstallResults)

	gorillalog.Info("Processing managed updates...")
	updateResults := processUpdateResultsFunc(updates, ctx.catalogs, ctx.cfg.URLPackages, ctx.cfg.CachePath, ctx.cfg.CheckOnly)
	logManagedResultFailures("update", updateResults)

	gorillalog.Info("Saving GorillaReport.json...")
	if ctx.cfg.CheckOnly {
		report.Print()
	}

	gorillalog.Info("Cleaning up the cache...")
	processCleanUpFunc(ctx.cfg.CachePath)

	gorillalog.Info("Done!")
	return nil
}

// managedItemRun executes one accepted App Catalog mutation. Its work set is
// limited to the requested item and work causally required by that item, such as
// install dependencies. It must never perform unrelated managed convergence.
func managedItemRun(cfg config.Configuration, requestedItem, requestedAction string) (installer.Result, error) {
	if requestedAction != "InstallItem" && requestedAction != "RemoveItem" {
		return installer.Result{}, fmt.Errorf("unsupported targeted managed item action %q", requestedAction)
	}

	if err := prepareManagedEnvironment(cfg, true); err != nil {
		return installer.Result{}, err
	}

	if !cfg.CheckOnly {
		report.Start()
		defer report.End()
	}

	ctx, err := prepareManagedExecution(cfg)
	if err != nil {
		return installer.Result{}, err
	}

	defer func() {
		gorillalog.Info("Cleaning up the cache...")
		processCleanUpFunc(ctx.cfg.CachePath)
		gorillalog.Info("Done!")
	}()

	switch requestedAction {
	case "InstallItem":
		gorillalog.Info("Processing targeted managed install:", requestedItem)
		results := processInstallResultsFunc([]string{requestedItem}, ctx.catalogs, ctx.cfg.URLPackages, ctx.cfg.CachePath, ctx.cfg.CheckOnly)
		logManagedResultFailures("install", results)
		if result, ok := findManagedItemResult(results, requestedItem); ok {
			return result, nil
		}
		return installer.Result{}, fmt.Errorf("targeted InstallItem returned no result for %q", requestedItem)

	case "RemoveItem":
		gorillalog.Info("Processing targeted managed uninstall:", requestedItem)
		results := processUninstallResultsFunc([]string{requestedItem}, ctx.catalogs, ctx.cfg.URLPackages, ctx.cfg.CachePath, ctx.cfg.CheckOnly)
		logManagedResultFailures("uninstall", results)
		if result, ok := findManagedItemResult(results, requestedItem); ok {
			return result, nil
		}
		return installer.Result{}, fmt.Errorf("targeted RemoveItem returned no result for %q", requestedItem)
	}

	return installer.Result{}, fmt.Errorf("unsupported targeted managed item action %q", requestedAction)
}

// logManagedResultFailures makes result-aware failures observable in ordinary
// CLI and scheduled convergence, not only to App Catalog callers retaining one
// requested result. This is especially important for synthetic process failures
// such as dependency_failed, dependency_cycle, and invalid_catalog_item that do
// not necessarily reach the installer/report path.
func logManagedResultFailures(phase string, results []process.ItemResult) {
	for _, itemResult := range results {
		result := itemResult.Result
		if result.Outcome != installer.OutcomeFailed {
			continue
		}

		fields := []interface{}{"Managed", phase, "failed:", itemResult.ItemName}
		if result.ErrorCode != "" {
			fields = append(fields, "code=", result.ErrorCode)
		}
		if result.Message != "" {
			fields = append(fields, "message=", result.Message)
		}
		managedResultWarnFunc(fields...)
	}
}

func findManagedItemResult(results []process.ItemResult, itemName string) (installer.Result, bool) {
	for i := len(results) - 1; i >= 0; i-- {
		if results[i].ItemName == itemName {
			return results[i].Result, true
		}
	}
	return installer.Result{}, false
}
