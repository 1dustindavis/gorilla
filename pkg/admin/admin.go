package admin

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	"gopkg.in/yaml.v2"
)

// BuildCatalogs iterates through the files in the `packages-info` directory and attempts to compile them into catalog files
func BuildCatalogs(repoPath string) error {

	// Iterate through `packages-info` to find the files we need to parse
	var packageInfoQueue []string
	packagesInfoPath := filepath.Join(repoPath, "packages-info")
	catalogsPath := filepath.Join(repoPath, "catalogs")
	err := filepath.Walk(packagesInfoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			gorillalog.Warn("Failed to access path:", path, err)
		}
		// If not a directory, add the path to a slice "packageInfoQueue"
		if filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml" {
			gorillalog.Debug("Queuing package-info file to be parsed:", path)
			packageInfoQueue = append(packageInfoQueue, path)
		}
		return nil
	})

	if err != nil {
		gorillalog.Warn("Error walking path:", packagesInfoPath, err)
		return err
	}

	// Parse the files, adding each item to a new map of catalogs
	// map["catalogName"]map[catalog.Item]
	var catalogMap = make(map[string]map[string]catalog.Item)
	for _, packageInfoPath := range packageInfoQueue {

		// Read the packageInfo file
		yamlFile, err := ioutil.ReadFile(packageInfoPath)
		if err != nil {
			gorillalog.Warn("Error reading package-info file:", err)
			return err
		}

		// Parse the packageInfo yaml
		var packageInfo catalog.Item
		err = yaml.Unmarshal(yamlFile, &packageInfo)
		if err != nil {
			gorillalog.Warn("Unable to parse package-info yaml:", err)
			return err
		}

		// Add the new parsed item to the catalogMap
		if packageInfo.Catalog == "" {
			gorillalog.Warn("No catalog defined in package-info:", packageInfo.DisplayName)
			continue
		}
		if catalogMap[packageInfo.Catalog] == nil {
			catalogMap[packageInfo.Catalog] = map[string]catalog.Item{}
		}
		catalogMap[packageInfo.Catalog][packageInfo.DisplayName] = packageInfo
	}

	// Check if the catalogs directory exists
	if _, err := os.Stat(catalogsPath); !os.IsNotExist(err) {
		gorillalog.Info("Cleaning existing catalogs directory:", catalogsPath)
		os.RemoveAll(catalogsPath)
	}

	// create the catalogs directory
	err = os.MkdirAll(catalogsPath, 0755)
	if err != nil {
		gorillalog.Warn("Unable to make filepath:", catalogsPath, err)
		return err
	}

	// For each catalog, save all items to disk as a new catalog file in <RepoPath>/catalogs/<catalogName>
	for catalogName, catalogItems := range catalogMap {

		// Convert the map of catalog items to yaml format
		catalogYaml, err := yaml.Marshal(catalogItems)
		if err != nil {
			gorillalog.Warn("Unable to generate catalog:", catalogName, err)
		}

		// Determine the path where the yaml file should be saved
		catalogPath := filepath.Join(catalogsPath, catalogName+".yaml")

		// Create the file
		f, err := os.Create(filepath.Clean(catalogPath))
		if err != nil {
			gorillalog.Warn("Unable to create catalog:", catalogPath, err)
		}
		defer f.Close()

		// Write the catalogYaml to the file we opened
		_, err = f.Write(catalogYaml)
		if err != nil {
			gorillalog.Warn("Unable to write catalog to disk:", catalogPath, err)
			return err
		}
	}
	return nil
}

// ImportItem attempts to create a yaml file in the `packages-info` directory
func ImportItem(repoPath string, itemPath string) error {
	return fmt.Errorf("Import is not yet implemented")
}
