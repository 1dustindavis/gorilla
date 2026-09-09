package process

import (
	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/installer"
)

// ManagedInstallResults preserves the legacy managed-run install semantics while
// retaining structured per-item outcomes. Each selected item processes only its
// direct dependencies, and dependencies are not deduplicated across selections.
// This is intentionally distinct from InstallResults, which is the recursive
// item-scoped execution path guarded by the App Catalog activation gate.
func ManagedInstallResults(installs []string, catalogsMap map[int]map[string]catalog.Item, urlPackages, cachePath string, checkOnly bool) []ItemResult {
	results := make([]ItemResult, 0, len(installs))
	for _, itemName := range installs {
		item, _, ok := ResolveItem(itemName, catalogsMap)
		if !ok {
			continue
		}
		for _, dependencyName := range item.Dependencies {
			dependency, _, ok := ResolveItem(dependencyName, catalogsMap)
			if !ok {
				continue
			}
			result := installer.InstallResult(dependency, "install", urlPackages, cachePath, checkOnly)
			results = append(results, ItemResult{ItemName: dependencyName, Result: result})
		}
		result := installer.InstallResult(item, "install", urlPackages, cachePath, checkOnly)
		results = append(results, ItemResult{ItemName: itemName, Result: result})
	}
	return results
}
