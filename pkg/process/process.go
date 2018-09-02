package process

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	"github.com/1dustindavis/gorilla/pkg/installer"
	"github.com/1dustindavis/gorilla/pkg/manifest"
)

//Manifests iterates though the first manifest and any included manifests
func Manifests(manifests []manifest.Item, catalog map[string]catalog.Item) (installs, uninstalls, updates []string) {
	// Compile all of the installs, uninstalls, and updates into arrays
	for _, manifestItem := range manifests {
		// Installs
		for _, item := range manifestItem.Installs {
			if item != "" && catalog[item].InstallerItemLocation != "" {
				installs = append(installs, item)
			}
		}
		// Uninstalls
		for _, item := range manifestItem.Uninstalls {
			if item != "" {
				uninstalls = append(uninstalls, item)
			}
		}
		// Updates
		for _, item := range manifestItem.Updates {
			if item != "" {
				updates = append(updates, item)
			}
		}
	}
	return
}

// Installs prepares and then installs and array of items
func Installs(installs []string, catalog map[string]catalog.Item) {
	// Iterate through the installs array, install dependencies, and then the item itself
	for _, item := range installs {
		// Check for dependencies and install if found
		if len(catalog[item].Dependencies) > 0 {
			for _, dependency := range catalog[item].Dependencies {
				installer.Install(catalog[dependency])
			}
		}
		// Install the item
		installer.Install(catalog[item])
	}
}

// Uninstalls prepares and then installs and array of items
func Uninstalls(uninstalls []string, catalog map[string]catalog.Item) {
	// Iterate through the uninstalls array and uninstall the item
	for _, item := range uninstalls {
		// Uninstall the item
		installer.Uninstall(catalog[item])
	}
}

// Updates prepares and then installs and array of items
func Updates(updates []string, catalog map[string]catalog.Item) {
	// Iterate through the updates array and update the item **if it is already installed**
	for _, item := range updates {
		// Update the item
		installer.Update(catalog[item])
	}
}

func dirEmpty(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	// read in ONLY one file
	_, err = f.Readdir(1)

	// and if the file is EOF... well, the dir is empty.
	if err == io.EOF {
		return true
	}
	return false
}

func fileOld(info os.FileInfo) bool {
	// Age of the file
	fileAge := time.Since(info.ModTime())

	// Our limit
	days := 5

	// Convert from days
	hours := days * 24
	ageLimit := time.Duration(hours) * time.Hour

	// If the file is older than our limit, delete it
	if fileAge > ageLimit {
		return true
	}

	return false
}

// CleanUp checks the age of items in the cache and removes if older than 10 days
func CleanUp() {

	// Clean up old files
	err := filepath.Walk(config.CachePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			gorillalog.Warn("Failed to access path:", path, err)
			return err
		}
		// If not a directory and older that our limit, delete
		if !info.IsDir() && fileOld(info) {
			gorillalog.Info("Cleaning old cached file:", info.Name())
			os.Remove(path)
			return nil
		}
		return nil
	})
	if err != nil {
		gorillalog.Warn("error walking path:", config.CachePath, err)
		return
	}

	// Clean up empty directories
	err = filepath.Walk(config.CachePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			gorillalog.Warn("Failed to access path:", path, err)
			return err
		}

		// If a dir and empty, delete
		if info.IsDir() && dirEmpty(path) {
			gorillalog.Info("Cleaning empty directory:", info.Name())
			os.Remove(path)
			return nil

		}
		return nil
	})
	if err != nil {
		gorillalog.Warn("error walking path:", config.CachePath, err)
		return
	}
}
