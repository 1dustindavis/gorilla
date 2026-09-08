package process

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	"github.com/1dustindavis/gorilla/pkg/installer"
	"github.com/1dustindavis/gorilla/pkg/manifest"
)

// ResolveItem searches catalogs in configured order and returns the first item
// with enough installer or uninstaller metadata to be actionable. The returned
// map key identifies the catalog that supplied the item. Managed processing and
// App Catalog both use this resolver so duplicate names and invalid definitions
// follow the same precedence rules.
func ResolveItem(itemName string, catalogsMap map[int]map[string]catalog.Item) (catalog.Item, int, bool) {
	// Map iteration is unordered, so sort catalog keys before resolving.
	keys := make([]int, 0)
	for k := range catalogsMap {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	var invalidReasons []string

	// The first valid definition wins. Keep invalid-definition details so a
	// missing item can be distinguished from an unusable one in diagnostics.
	for _, k := range keys {
		if item, exists := catalogsMap[k][itemName]; exists {
			validInstallItem := (item.Installer.Type != "" && item.Installer.Location != "")
			validUninstallItem := (item.Uninstaller.Type != "" && item.Uninstaller.Location != "") ||
				item.Uninstaller.Type == "msix" ||
				item.Installer.Type == "msix"

			if validInstallItem || validUninstallItem {
				return item, k, true
			}

			missing := []string{}
			if item.Installer.Type == "" {
				missing = append(missing, "installer.type")
			}
			if item.Installer.Location == "" {
				missing = append(missing, "installer.location")
			}
			if item.Uninstaller.Type == "" {
				missing = append(missing, "uninstaller.type")
			}
			if item.Uninstaller.Location == "" {
				missing = append(missing, "uninstaller.location")
			}
			invalidReasons = append(invalidReasons, fmt.Sprintf("catalog index %d missing required fields: %s", k, strings.Join(missing, ", ")))
		}
	}

	if len(invalidReasons) > 0 {
		gorillalog.Warn(fmt.Sprintf(
			"skipping catalog item %q because it is missing required installer/uninstaller type/location fields (%s)",
			itemName,
			strings.Join(invalidReasons, "; "),
		))
		return catalog.Item{}, 0, false
	}
	gorillalog.Warn(fmt.Sprintf("skipping item %q because it was not found in any catalog", itemName))
	return catalog.Item{}, 0, false

}

func firstItem(itemName string, catalogsMap map[int]map[string]catalog.Item) (catalog.Item, bool) {
	item, _, ok := ResolveItem(itemName, catalogsMap)
	return item, ok
}

// Manifests iterates though the first manifest and any included manifests
func Manifests(manifests []manifest.Item, catalogsMap map[int]map[string]catalog.Item) (installs, uninstalls, updates []string) {
	// Compile all of the installs, uninstalls, and updates into arrays
	for _, manifestItem := range manifests {
		// Installs
		for _, item := range manifestItem.Installs {
			// Check for the first valid item from our catalogs
			// Continue to the next item in the loop if we get an error
			if _, ok := firstItem(item, catalogsMap); !ok {
				continue
			}

			// If we didnt error, append the item to our installs list
			installs = append(installs, item)
		}
		// Uninstalls
		for _, item := range manifestItem.Uninstalls {
			// Check for the first valid item from our catalogs
			// Continue to the next item in the loop if we get an error
			if _, ok := firstItem(item, catalogsMap); !ok {
				continue
			}

			// If we didnt error, append the item to our uninstalls list
			uninstalls = append(uninstalls, item)
		}
		// Updates
		for _, item := range manifestItem.Updates {
			// Check for the first valid item from our catalogs
			// Continue to the next item in the loop if we get an error
			if _, ok := firstItem(item, catalogsMap); !ok {
				continue
			}

			// If we didnt error, append the item to our updates list
			updates = append(updates, item)
		}
	}
	return
}

// This abstraction allows us to override when testing
var installerInstall = installer.Install

// ItemResult preserves the catalog key that was requested as well as the
// installer outcome. Display names are presentation metadata and are not a
// stable identifier for service operations.
type ItemResult struct {
	ItemName string
	Result   installer.Result
}

var installerInstallResult = installer.InstallResult

// Installs prepares and then installs an array of items
func Installs(installs []string, catalogsMap map[int]map[string]catalog.Item, urlPackages, cachePath string, CheckOnly bool) {
	executeInstallClosure(installs, catalogsMap, func(itemName string, item catalog.Item) {
		installerInstall(item, "install", urlPackages, cachePath, CheckOnly)
	})
}

// InstallResults executes each requested item and its transitive dependencies
// once, in dependency-first order. A parent is not executed when a dependency
// cannot be resolved or fails. The legacy Installs function uses the same
// closure while discarding results for CLI compatibility.
func InstallResults(installs []string, catalogsMap map[int]map[string]catalog.Item, urlPackages, cachePath string, checkOnly bool) []ItemResult {
	results := make([]ItemResult, 0, len(installs))
	state := make(map[string]visitState)

	var execute func(string) installer.Result
	execute = func(itemName string) installer.Result {
		switch state[itemName] {
		case visitDone:
			return installer.Result{ItemName: itemName, Action: "install", Outcome: installer.OutcomeAlreadyCurrent, Message: "Dependency already processed"}
		case visitActive:
			return installer.Result{ItemName: itemName, Action: "install", Outcome: installer.OutcomeFailed, ErrorCode: "dependency_cycle", Message: "Dependency cycle detected"}
		}

		item, ok := firstItem(itemName, catalogsMap)
		if !ok || item.Installer.Type == "" || item.Installer.Location == "" {
			result := installer.Result{ItemName: itemName, Action: "install", Outcome: installer.OutcomeFailed, ErrorCode: "invalid_dependency", Message: "Catalog item has no valid installer"}
			results = append(results, ItemResult{ItemName: itemName, Result: result})
			state[itemName] = visitDone
			return result
		}

		state[itemName] = visitActive
		for _, dependency := range item.Dependencies {
			dependencyResult := execute(dependency)
			if dependencyResult.Outcome == installer.OutcomeFailed {
				result := installer.Result{ItemName: itemName, Action: "install", Outcome: installer.OutcomeFailed, ErrorCode: "dependency_failed", Message: fmt.Sprintf("Dependency %s did not complete: %s", dependency, dependencyResult.Message)}
				results = append(results, ItemResult{ItemName: itemName, Result: result})
				state[itemName] = visitDone
				return result
			}
		}

		result := installerInstallResult(item, "install", urlPackages, cachePath, checkOnly)
		if result.ItemName == "" {
			result.ItemName = item.DisplayName
		}
		if result.Action == "" {
			result.Action = "install"
		}
		results = append(results, ItemResult{ItemName: itemName, Result: result})
		state[itemName] = visitDone
		return result
	}

	for _, itemName := range installs {
		if state[itemName] != visitDone {
			execute(itemName)
		}
	}
	return results
}

type visitState uint8

const (
	visitNone visitState = iota
	visitActive
	visitDone
)

func executeInstallClosure(installs []string, catalogsMap map[int]map[string]catalog.Item, execute func(string, catalog.Item)) {
	state := make(map[string]visitState)
	var visit func(string)
	visit = func(itemName string) {
		if state[itemName] != visitNone {
			return
		}
		item, ok := firstItem(itemName, catalogsMap)
		if !ok || item.Installer.Type == "" || item.Installer.Location == "" {
			state[itemName] = visitDone
			return
		}
		state[itemName] = visitActive
		for _, dependency := range item.Dependencies {
			visit(dependency)
		}
		state[itemName] = visitDone
		execute(itemName, item)
	}
	for _, itemName := range installs {
		visit(itemName)
	}
}

// Uninstalls prepares and then installs an array of items
func Uninstalls(uninstalls []string, catalogsMap map[int]map[string]catalog.Item, urlPackages, cachePath string, CheckOnly bool) {
	// Iterate through the uninstalls array and uninstall the item
	for _, item := range uninstalls {
		// Get the first valid item from our catalogs
		// Continue to the next item in the loop if we get an error
		validItem, ok := firstItem(item, catalogsMap)
		if !ok {
			continue
		}
		// Uninstall the item
		installerInstall(validItem, "uninstall", urlPackages, cachePath, CheckOnly)
	}
}

// UninstallResults reports every selected uninstall attempt. It deliberately
// does not infer success from the managed run as a whole.
func UninstallResults(uninstalls []string, catalogsMap map[int]map[string]catalog.Item, urlPackages, cachePath string, checkOnly bool) []ItemResult {
	return actionResults(uninstalls, "uninstall", catalogsMap, urlPackages, cachePath, checkOnly)
}

// Updates prepares and then installs an array of items
func Updates(updates []string, catalogsMap map[int]map[string]catalog.Item, urlPackages, cachePath string, CheckOnly bool) {
	// Iterate through the updates array and update the item **if it is already installed**
	for _, item := range updates {
		// Get the first valid item from our catalogs
		// Continue to the next item in the loop if we get an error
		validItem, ok := firstItem(item, catalogsMap)
		if !ok {
			continue
		}
		// Update the item
		installerInstall(validItem, "update", urlPackages, cachePath, CheckOnly)
	}
}

// UpdateResults reports every selected update attempt. Dependency installation
// remains the responsibility of install selections; an update does not silently
// install an otherwise unselected dependency tree.
func UpdateResults(updates []string, catalogsMap map[int]map[string]catalog.Item, urlPackages, cachePath string, checkOnly bool) []ItemResult {
	return actionResults(updates, "update", catalogsMap, urlPackages, cachePath, checkOnly)
}

func actionResults(items []string, action string, catalogsMap map[int]map[string]catalog.Item, urlPackages, cachePath string, checkOnly bool) []ItemResult {
	results := make([]ItemResult, 0, len(items))
	for _, itemName := range items {
		item, ok := firstItem(itemName, catalogsMap)
		if !ok || !actionableFor(item, action) {
			results = append(results, ItemResult{
				ItemName: itemName,
				Result: installer.Result{
					ItemName:  itemName,
					Action:    action,
					Outcome:   installer.OutcomeFailed,
					ErrorCode: "invalid_catalog_item",
					Message:   "Catalog item is not actionable",
				},
			})
			continue
		}
		result := installerInstallResult(item, action, urlPackages, cachePath, checkOnly)
		if result.ItemName == "" {
			result.ItemName = item.DisplayName
		}
		if result.Action == "" {
			result.Action = action
		}
		results = append(results, ItemResult{ItemName: itemName, Result: result})
	}
	return results
}

func actionableFor(item catalog.Item, action string) bool {
	switch action {
	case "install", "update":
		return item.Installer.Type != "" && item.Installer.Location != ""
	case "uninstall":
		return (item.Uninstaller.Type != "" && item.Uninstaller.Location != "") ||
			item.Uninstaller.Type == "msix" || item.Installer.Type == "msix"
	default:
		return false
	}
}

// dirEmpty returns true if the directory is empty
func dirEmpty(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	// Try to get the first item in the directory
	_, err = f.Readdir(1)

	// If the we recevie an EOF error, the dir is empty
	return err == io.EOF
}

// fileOld returns true if the file is older than
// the limit defined in the variable `days`
func fileOld(info os.FileInfo) bool {
	// Age of the file
	fileAge := time.Since(info.ModTime())

	// Our limit
	days := 5

	// Convert from days
	hours := days * 24
	ageLimit := time.Duration(hours) * time.Hour

	// If the file is older than our limit, return true
	return fileAge > ageLimit
}

// This abstraction allows us to override when testing
var osRemove = os.Remove

// CleanUp checks the age of items in the cache and removes if older than 10 days
func CleanUp(cachePath string) {

	// Clean up old files
	err := filepath.Walk(cachePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			gorillalog.Warn("Failed to access path:", path, err)
			return err
		}
		// If not a directory and older that our limit, delete
		if !info.IsDir() && fileOld(info) {
			gorillalog.Info("Cleaning old cached file:", info.Name())
			osRemove(path)
			return nil
		}
		return nil
	})
	if err != nil {
		gorillalog.Warn("error walking path:", cachePath, err)
		return
	}

	// Clean up empty directories
	err = filepath.Walk(cachePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			gorillalog.Warn("Failed to access path:", path, err)
			return err
		}

		// If a dir and empty, delete
		if info.IsDir() && dirEmpty(path) {
			gorillalog.Info("Cleaning empty directory:", info.Name())
			osRemove(path)
			return nil

		}
		return nil
	})
	if err != nil {
		gorillalog.Warn("error walking path:", cachePath, err)
		return
	}
}
