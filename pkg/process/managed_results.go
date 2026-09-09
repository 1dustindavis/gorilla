package process

import (
	"fmt"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/installer"
)

// ManagedInstallResults preserves the legacy managed-run install semantics while
// retaining structured per-item outcomes. Each selected item processes only its
// direct dependencies, dependencies are not deduplicated across selections, and
// the parent still executes after a dependency problem just as legacy Installs
// does. The returned parent result nevertheless records a necessary dependency
// failure so service consumers do not mistake parent execution for success.
// This is intentionally distinct from InstallResults, which is the recursive
// item-scoped execution path guarded by the App Catalog activation gate.
func ManagedInstallResults(installs []string, catalogsMap map[int]map[string]catalog.Item, urlPackages, cachePath string, checkOnly bool) []ItemResult {
	results := make([]ItemResult, 0, len(installs))
	for _, itemName := range installs {
		item, _, ok := ResolveItem(itemName, catalogsMap)
		if !ok {
			continue
		}

		var failedDependency *ItemResult
		for _, dependencyName := range item.Dependencies {
			dependency, _, ok := ResolveItem(dependencyName, catalogsMap)
			if !ok {
				result := ItemResult{ItemName: dependencyName, Result: installer.Result{
					ItemName:  dependencyName,
					Action:    "install",
					Outcome:   installer.OutcomeFailed,
					ErrorCode: "invalid_dependency",
					Message:   "Catalog dependency is missing or invalid",
				}}
				results = append(results, result)
				if failedDependency == nil {
					copy := result
					failedDependency = &copy
				}
				continue
			}
			result := ItemResult{ItemName: dependencyName, Result: installerInstallResult(dependency, "install", urlPackages, cachePath, checkOnly)}
			results = append(results, result)
			if result.Result.Outcome == installer.OutcomeFailed && failedDependency == nil {
				copy := result
				failedDependency = &copy
			}
		}

		parentResult := installerInstallResult(item, "install", urlPackages, cachePath, checkOnly)
		if parentResult.Outcome != installer.OutcomeFailed && failedDependency != nil {
			parentResult = installer.Result{
				ItemName:  item.DisplayName,
				Action:    "install",
				Outcome:   installer.OutcomeFailed,
				ErrorCode: "dependency_failed",
				Message: fmt.Sprintf(
					"Dependency %s did not complete (%s): %s",
					failedDependency.ItemName,
					failedDependency.Result.ErrorCode,
					failedDependency.Result.Message,
				),
			}
		}
		results = append(results, ItemResult{ItemName: itemName, Result: parentResult})
	}
	return results
}

// ManagedActionResults mirrors the legacy managed-run update/uninstall loops
// while retaining structured results. It intentionally performs only catalog
// resolution before calling the installer boundary, matching Updates/Uninstalls
// rather than the stricter item-scoped actionResults validation.
func ManagedActionResults(items []string, action string, catalogsMap map[int]map[string]catalog.Item, urlPackages, cachePath string, checkOnly bool) []ItemResult {
	results := make([]ItemResult, 0, len(items))
	for _, itemName := range items {
		item, _, ok := ResolveItem(itemName, catalogsMap)
		if !ok {
			continue
		}
		result := installerInstallResult(item, action, urlPackages, cachePath, checkOnly)
		results = append(results, ItemResult{ItemName: itemName, Result: result})
	}
	return results
}
