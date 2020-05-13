package process

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	"github.com/1dustindavis/gorilla/pkg/installer"
	"github.com/1dustindavis/gorilla/pkg/manifest"
)

// firstItem returns the first occurrence of an item in a map of catalogs
func firstItem(itemName string, catalogsMap map[int]map[string]catalog.Item) (catalog.Item, error) {
	// Get the keys in the map and sort them so we can loop over them in order
	keys := make([]int, 0)
	for k := range catalogsMap {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	// loop through each catalog and return if we find a match
	for _, k := range keys {
		// If
		if item, exists := catalogsMap[k][itemName]; exists {
			// If it does exist, we should confirm it is a valid item
			validInstallItem := (item.Installer.Type != "" && item.Installer.Location != "")
			validUninstallItem := (item.Uninstaller.Type != "" && item.Uninstaller.Location != "")

			if validInstallItem || validUninstallItem {
				return item, nil
			}
		}
	}

	// return an empty catalog item if we didnt already find and return a match
	return catalog.Item{}, fmt.Errorf("did not find a valid item in any catalog; Item name: %v", itemName)

}

// Manifests iterates though the first manifest and any included manifests
func Manifests(manifests []manifest.Item, catalogsMap map[int]map[string]catalog.Item) (installs, uninstalls, updates []string) {
	// Compile all of the installs, uninstalls, and updates into arrays
	for _, manifestItem := range manifests {
		// Installs
		for _, item := range manifestItem.Installs {
			// Check for the first valid item from our catalogs
			// Continue to the next item in the loop if we get an error
			_, err := firstItem(item, catalogsMap)
			if err != nil {
				gorillalog.Warn(err)
				continue
			}

			// If we didnt error, append the item to our installs list
			installs = append(installs, item)
		}
		// Uninstalls
		for _, item := range manifestItem.Uninstalls {
			// Check for the first valid item from our catalogs
			// Continue to the next item in the loop if we get an error
			_, err := firstItem(item, catalogsMap)
			if err != nil {
				gorillalog.Warn(err)
				continue
			}

			// If we didnt error, append the item to our uninstalls list
			uninstalls = append(uninstalls, item)
		}
		// Updates
		for _, item := range manifestItem.Updates {
			// Check for the first valid item from our catalogs
			// Continue to the next item in the loop if we get an error
			_, err := firstItem(item, catalogsMap)
			if err != nil {
				gorillalog.Warn(err)
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

// Installs prepares and then installs an array of items
func Installs(installs []string, catalogsMap map[int]map[string]catalog.Item, urlPackages, cachePath string, CheckOnly bool) {
	// Iterate through the installs array, install dependencies, and then the item itself
	for _, item := range installs {
		// Get the first valid item from our catalogs
		// Continue to the next item in the loop if we get an error
		validItem, err := firstItem(item, catalogsMap)
		if err != nil {
			gorillalog.Warn(err)
			continue
		}
		// Check for dependencies and install if found
		if len(validItem.Dependencies) > 0 {
			for _, dependency := range validItem.Dependencies {
				validDependency, err := firstItem(dependency, catalogsMap)
				if err != nil {
					gorillalog.Warn(err)
					continue
				}
				installerInstall(validDependency, "install", urlPackages, cachePath, CheckOnly)
			}
		}
		// Install the item
		installerInstall(validItem, "install", urlPackages, cachePath, CheckOnly)
	}
}

// Uninstalls prepares and then installs an array of items
func Uninstalls(uninstalls []string, catalogsMap map[int]map[string]catalog.Item, urlPackages, cachePath string, CheckOnly bool) {
	// Iterate through the uninstalls array and uninstall the item
	for _, item := range uninstalls {
		// Get the first valid item from our catalogs
		// Continue to the next item in the loop if we get an error
		validItem, err := firstItem(item, catalogsMap)
		if err != nil {
			gorillalog.Warn(err)
			continue
		}
		// Uninstall the item
		installerInstall(validItem, "uninstall", urlPackages, cachePath, CheckOnly)
	}
}

// Updates prepares and then installs an array of items
func Updates(updates []string, catalogsMap map[int]map[string]catalog.Item, urlPackages, cachePath string, CheckOnly bool) {
	// Iterate through the updates array and update the item **if it is already installed**
	for _, item := range updates {
		// Get the first valid item from our catalogs
		// Continue to the next item in the loop if we get an error
		validItem, err := firstItem(item, catalogsMap)
		if err != nil {
			gorillalog.Warn(err)
			continue
		}
		// Update the item
		installerInstall(validItem, "update", urlPackages, cachePath, CheckOnly)
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
