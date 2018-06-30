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
	DisplayName           string   `yaml:"display_name"`
	InstallerItemLocation string   `yaml:"installer_item_location"`
	InstallerItemHash     string   `yaml:"installer_item_hash"`
	Version               string   `yaml:"version"`
	Dependencies          []string `yaml:"dependencies"`
}

// Get returns a map of Items from the catalog
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
