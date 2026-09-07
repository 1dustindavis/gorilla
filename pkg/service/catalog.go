package service

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/1dustindavis/gorilla/pkg/appcatalog"
	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/process"
	"github.com/1dustindavis/gorilla/pkg/status"
)

var (
	catalogGet    = catalog.Get
	statusObserve = status.Observe
)

type optionalItemDetails struct {
	Contract           appcatalog.Item
	InstallerType      string
	InstallerPackageID string
	InstallerLocation  string
}

type resolvedCatalogItem struct {
	item    catalog.Item
	catalog string
	found   bool
}

func resolveCatalogItem(name string, catalogs map[int]map[string]catalog.Item, catalogNames []string) resolvedCatalogItem {
	item, key, found := process.ResolveItem(name, catalogs)
	if !found {
		return resolvedCatalogItem{}
	}
	catalogName := ""
	if key > 0 && key <= len(catalogNames) {
		catalogName = catalogNames[key-1]
	}
	return resolvedCatalogItem{item: item, catalog: catalogName, found: true}
}

func getOptionalItemDetails(cfg config.Configuration) ([]optionalItemDetails, error) {
	manifests, extraCatalogs, err := manifestGet(cfg)
	if err != nil {
		return nil, fmt.Errorf("retrieve manifests: %w", err)
	}
	for _, name := range extraCatalogs {
		if !slices.Contains(cfg.Catalogs, name) {
			cfg.Catalogs = append(cfg.Catalogs, name)
		}
	}
	catalogs, err := catalogGet(cfg)
	catalogDetailCode := "catalog_item_missing_or_invalid"
	if err != nil {
		// Optional assignments are still useful when catalog retrieval fails.
		// Return honest unavailable entries instead of failing the whole list or
		// inventing absence. Mutations will be denied by their actions.
		catalogs = map[int]map[string]catalog.Item{}
		catalogDetailCode = "catalog_load_failed"
	}
	selection, err := loadServiceLocalManifest(cfg)
	if err != nil {
		return nil, err
	}
	status.ResetRegistryCache()

	optional := map[string]bool{}
	requiredInstalls := map[string]int{}
	requiredUninstalls := map[string]int{}
	for _, entry := range manifests {
		for _, name := range entry.OptionalInstalls {
			if strings.TrimSpace(name) != "" {
				optional[name] = true
			}
		}
		for _, name := range entry.Installs {
			requiredInstalls[name]++
		}
		for _, name := range entry.Uninstalls {
			requiredUninstalls[name]++
		}
	}
	// manifest.Get includes the service-owned manifest. Remove exactly its own
	// contribution so an administrator entry for the same item still wins.
	serviceManifestIncluded := false
	servicePath := filepath.Clean(serviceLocalManifestPath(cfg))
	for _, localPath := range cfg.LocalManifests {
		if filepath.Clean(localPath) == servicePath {
			serviceManifestIncluded = true
			break
		}
	}
	if serviceManifestIncluded {
		for _, name := range selection.Installs {
			requiredInstalls[name]--
		}
		for _, name := range selection.Uninstalls {
			requiredUninstalls[name]--
		}
	}

	dependencyRoots := map[string]bool{}
	for name, count := range requiredInstalls {
		dependencyRoots[name] = count > 0
	}
	for _, name := range selection.Installs {
		dependencyRoots[name] = true
	}
	dependencies := map[string]bool{}
	var visit func(string, map[string]bool)
	visit = func(name string, visiting map[string]bool) {
		if visiting[name] {
			return
		}
		visiting[name] = true
		resolved := resolveCatalogItem(name, catalogs, cfg.Catalogs)
		if resolved.found {
			for _, dependency := range resolved.item.Dependencies {
				dependencies[dependency] = true
				visit(dependency, visiting)
			}
		}
		delete(visiting, name)
	}
	for name, active := range dependencyRoots {
		if active {
			visit(name, map[string]bool{})
		}
	}

	names := make([]string, 0, len(optional))
	for name := range optional {
		names = append(names, name)
	}
	slices.Sort(names)
	details := make([]optionalItemDetails, 0, len(names))
	for _, name := range names {
		policy := appcatalog.Policy{
			Optional:           true,
			RequiredInstall:    requiredInstalls[name] > 0,
			RequiredUninstall:  requiredUninstalls[name] > 0,
			RequiredDependency: dependencies[name],
			Selection:          appcatalog.NoSelection,
		}
		if slices.Contains(selection.Installs, name) {
			policy.Selection = appcatalog.KeepInstalled
		}
		resolved := resolveCatalogItem(name, catalogs, cfg.Catalogs)
		if !resolved.found {
			observation := appcatalog.Observation{
				State:              appcatalog.Unknown,
				DetailCode:         catalogDetailCode,
				InstallRequirement: appcatalog.RequirementUnknown,
			}
			contract := appcatalog.Item{ItemName: name, DisplayName: name, Observation: observation, Policy: policy}
			contract.Actions = appcatalog.DecideActions(observation.State, policy, appcatalog.Capabilities{}, false)
			details = append(details, optionalItemDetails{Contract: contract})
			continue
		}

		observed, observeErr := statusObserve(resolved.item, "install", cfg.CachePath)
		state := appcatalog.ObservedState(observed.State)
		detailCode := observed.DetailCode
		if observeErr != nil {
			state, detailCode = appcatalog.DetectionFailed, "check_failed"
		}
		checked := observed.CheckedAtUTC
		observation := appcatalog.Observation{
			State:              state,
			InstalledVersion:   observed.InstalledVersion,
			CheckedAtUTC:       &checked,
			DetailCode:         detailCode,
			InstallRequirement: installRequirement(resolved.item, observed, observeErr),
		}
		canInstall := resolved.item.Installer.Type != "" && resolved.item.Installer.Location != ""
		canRemove := (resolved.item.Uninstaller.Type != "" && resolved.item.Uninstaller.Location != "") ||
			resolved.item.Uninstaller.Type == "msix" || resolved.item.Installer.Type == "msix"
		targetVersion := resolved.item.Version
		var target *string
		if targetVersion != "" {
			target = &targetVersion
		}
		displayName := resolved.item.DisplayName
		if displayName == "" {
			displayName = name
		}
		contract := appcatalog.Item{
			ItemName: name, DisplayName: displayName, Catalog: resolved.catalog,
			TargetVersion: target, Observation: observation, Policy: policy,
		}
		contract.Actions = appcatalog.DecideActionsWithRequirement(
			state,
			observation.InstallRequirement,
			policy,
			appcatalog.Capabilities{CanInstall: canInstall, CanRemove: canRemove},
			false,
		)
		details = append(details, optionalItemDetails{
			Contract: contract, InstallerType: resolved.item.Installer.Type,
			InstallerPackageID: resolved.item.Installer.PackageID,
			InstallerLocation:  resolved.item.Installer.Location,
		})
	}
	return details, nil
}

func installRequirement(item catalog.Item, observed status.Observation, observeErr error) appcatalog.RequirementState {
	if observeErr != nil || observed.State == status.DetectionFailed {
		return appcatalog.RequirementUnknown
	}
	// Check selection remains script, file, registry, then AppX. A script result
	// can establish requirement satisfaction even though it cannot prove presence.
	if item.Check.Script != "" {
		if observed.ActionNeeded {
			return appcatalog.RequirementNotSatisfied
		}
		return appcatalog.RequirementSatisfied
	}
	if observed.State == status.Unknown {
		return appcatalog.RequirementUnknown
	}
	if observed.ActionNeeded {
		return appcatalog.RequirementNotSatisfied
	}
	return appcatalog.RequirementSatisfied
}

func findOptionalItem(details []optionalItemDetails, name string) (optionalItemDetails, bool) {
	for _, item := range details {
		if item.Contract.ItemName == name {
			return item, true
		}
	}
	return optionalItemDetails{}, false
}
