package manifest

import (
	"fmt"
	"github.com/1dustindavis/gorilla/pkg/download"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Item represents a single object from the manifest
type Item struct {
	Name       string   `yaml:"name"`
	Includes   []string `yaml:"included_manifests"`
	Installs   []string `yaml:"managed_installs"`
	Uninstalls []string `yaml:"managed_uninstalls"`
	Upgrades   []string `yaml:"managed_upgrades"`
}

func getManifest(cachePath string, manifestName string) Item {
	// Get the yaml file and look for included_manifests
	yamlPath := filepath.Join(cachePath, manifestName) + ".yaml"
	yamlFile, err := ioutil.ReadFile(yamlPath)
	var manifest Item
	err = yaml.Unmarshal(yamlFile, &manifest)
	if err != nil {
		fmt.Println("Unable to parse yaml manifest:", yamlPath, err)
	}
	return manifest
}

// Get returns a slice the includes all manifest objects
func Get(cachePath string, manifest string, repoURL string) []Item {
	// Create a slice of all manifest objects
	var manifests []Item
	// Create a slice with the names of all manifests
	// This is so we can track them before we get the data
	var manifestsList []string

	// Setup interation tracking for manifests
	var manifestsTotal = len(manifestsList)
	var manifestsProcessed = 0
	var manifestsRemaining = 1

	// Add the top level manifest to the list
	manifestsList = append(manifestsList, manifest)

	for manifestsRemaining > 0 {
		currentManifest := manifestsList[manifestsProcessed]

		// Add the current manifest to our working list
		workingList := []string{currentManifest}

		// Download the manifest
		manifestURL := repoURL + "manifests/" + currentManifest + ".yaml"
		err := download.File(cachePath, manifestURL)
		if err != nil {
			fmt.Println("Unable to retrieve manifest:", currentManifest, err)
			os.Exit(1)
		}

		// Get new manifest
		newManifest := getManifest(cachePath, currentManifest)

		// Add any includes to our working list
		for _, item := range newManifest.Includes {
			workingList = append(workingList, item)
		}

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

		// Increment counters
		manifestsTotal = len(manifestsList)
		manifestsProcessed++
		manifestsRemaining = manifestsTotal - manifestsProcessed
	}
	return manifests
}
