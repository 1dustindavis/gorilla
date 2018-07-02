package catalog

import (
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"

	"github.com/1dustindavis/gorilla/pkg/download"
	"gopkg.in/yaml.v2"
)

// Item contains an individual entry from the catalog
type Item struct {
	Dependencies          []string `yaml:"dependencies"`
	DisplayName           string   `yaml:"display_name"`
	InstallerItemHash     string   `yaml:"installer_item_hash"`
	InstallerItemLocation string   `yaml:"installer_item_location"`
	InstallerType         string   `yaml:"installer_type"`
	UninstallMethod       string   `yaml:"uninstall_method"`
	Version               string   `yaml:"version"`
}

// Get returns a map of `Item` from the catalog
func Get(cachePath string, catalogName string, repoURL string) map[string]Item {

	// Download the catalog
	catalogURL := repoURL + "catalogs/" + catalogName + ".yaml"
	err := download.File(cachePath, catalogURL)
	if err != nil {
		fmt.Println("Unable to retrieve catalog:", catalogName, err)
		log.Fatal(err)
	}

	// Parse the catalog
	yamlPath := filepath.Join(cachePath, catalogName) + ".yaml"
	yamlFile, err := ioutil.ReadFile(yamlPath)
	var catalog map[string]Item
	err = yaml.Unmarshal(yamlFile, &catalog)
	if err != nil {
		fmt.Println("Unable to parse yaml catalog:", yamlPath, err)
	}
	return catalog
}
