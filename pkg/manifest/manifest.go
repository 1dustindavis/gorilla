package manifest

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/download"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	"github.com/1dustindavis/gorilla/pkg/report"
	"gopkg.in/yaml.v2"
)

// Item represents a single object from the manifest
type Item struct {
	Name       string   `yaml:"name"`
	Includes   []string `yaml:"included_manifests"`
	Installs   []string `yaml:"managed_installs"`
	Uninstalls []string `yaml:"managed_uninstalls"`
	Updates    []string `yaml:"managed_updates"`
	Catalogs   []string `yaml:"catalogs"`
}

// This abstraction allows us to override when testing
var downloadGet = download.Get

// Get returns two slices:
// 1) All manifest objects
// 2) Aditional catalogs that need to be added to the config
func Get(cfg config.Configuration) (manifests []Item, newCatalogs []string) {
	// Create a slice with the names of all manifests
	// This is so we can track them before we get the data
	var manifestsList []string

	// Setup iteration tracking for manifests
	var manifestsTotal int
	var manifestsProcessed = 0
	var manifestsRemaining = 1

	// Add the top level manifest to the list
	manifestsList = append(manifestsList, cfg.Manifest)

	// Setup to catch a potential failure
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
			report.End()
			os.Exit(1)
		}
	}()

	for manifestsRemaining > 0 {
		currentManifest := manifestsList[manifestsProcessed]

		// Add the current manifest to our working list
		workingList := []string{currentManifest}

		// Download the manifest
		manifestURL := cfg.URL + "manifests/" + currentManifest + ".yaml"
		gorillalog.Info("Manifest Url:", manifestURL)
		yamlFile, err := downloadGet(manifestURL)
		if err != nil {
			gorillalog.Error("Unable to retrieve manifest: ", err)
		}

		newManifest := parseManifest(manifestURL, yamlFile)

		// Add any includes to our working list
		workingList = append(workingList, newManifest.Includes...)

		// Get workingList unique items, and add to the real list
		for _, item := range workingList {

			// Check if unique in manifestsList
			var uniqueInList = true
			for i := range manifestsList {
				if manifestsList[i] == item {
					uniqueInList = false
				}
			}
			// Update manifestsList if it is unique
			if uniqueInList {
				manifestsList = append(manifestsList, item)
			}
		}

		// Check if this is unique in manifests
		var uniqueInManifests = true
		for i := range manifests {
			if manifests[i].Name == newManifest.Name {
				uniqueInManifests = false
			}
		}
		// Update manifests
		if uniqueInManifests {
			// manifests = append([]Item{newManifest}, manifests...)
			manifests = append(manifests, newManifest)
		}

		// If any catalogs are in the manifest, append them to the end of the list
		for _, newCatalog := range newManifest.Catalogs {
			// Before adding it, check if it is already on the list
			var match bool
			for _, oldCatalog := range cfg.Catalogs {
				if oldCatalog == newCatalog {
					match = true
				}
			}
			// If "match" is still false, it is not already in the catalog slice
			if !match {
				newCatalogs = append(newCatalogs, newCatalog)
			}
		}

		// Increment counters
		manifestsTotal = len(manifestsList)
		manifestsProcessed++
		manifestsRemaining = manifestsTotal - manifestsProcessed
	}

	// Add the local manifest after processing all other manifests
	if len(cfg.LocalManifests) > 0 {
		for _, manifest := range cfg.LocalManifests {
			var localManifest Item
			gorillalog.Info("Manifest File:", manifest)
			localManifestsYaml, err := ioutil.ReadFile(manifest)
			if err != nil {
				gorillalog.Error("Unable to parse yaml manifest: ", manifest, err)
			}
			localManifest = parseManifest(manifest, localManifestsYaml)
			manifests = append(manifests, localManifest)
		}
	}

	return manifests, newCatalogs
}

func parseManifest(manifestURL string, yamlFile []byte) Item {
	// Parse the new manifest
	var newManifest Item
	err := yaml.Unmarshal(yamlFile, &newManifest)
	if err != nil {
		gorillalog.Error("Unable to parse yaml manifest: ", manifestURL, err)
	}
	return newManifest
}
