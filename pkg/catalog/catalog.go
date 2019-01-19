package catalog

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/download"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	"gopkg.in/yaml.v2"
)

// Item contains an individual entry from the catalog
type Item struct {
	Dependencies []string      `yaml:"dependencies"`
	DisplayName  string        `yaml:"display_name"`
	Check        InstallCheck  `yaml:"check"`
	Installer    InstallerItem `yaml:"installer"`
	Uninstaller  InstallerItem `yaml:"uninstaller"`
	Version      string        `yaml:"version"`
}

// InstallerItem holds information about how to install a catalog item
type InstallerItem struct {
	Type      string   `yaml:"type"`
	Location  string   `yaml:"location"`
	Hash      string   `yaml:"hash"`
	Arguments []string `yaml:"arguments"`
}

// InstallCheck holds information about how to check the status of a catalog item
type InstallCheck struct {
	Path     PathCheck `yaml:"path"`
	Script   string    `yaml:"script"`
	Registry RegCheck  `yaml:"registry"`
}

// PathCheck holds information about checking via path
type PathCheck struct {
	Path        string `yaml:"path"`
	Version     string `yaml:"version"`
	ProductName string `yaml:"product_name"`
	Hash        string `yaml:"hash"`
}

// RegCheck holds information about checking via registry
type RegCheck struct {
	DisplayName    string `yaml:"display_name"`
	DisplayVersion string `yaml:"display_version"`
}

var downloadFile = download.File

// Get returns a map of `Item` from the catalog
func Get() map[int]map[string]Item {

	// catalogMap is an map of parsed catalogs
	var catalogMap = make(map[int]map[string]Item)

	// catalogCount allows us to be sure we are processing catalogs in order
	var catalogCount = 0

	// Error if dont have at least one catalog
	if len(config.Current.Catalogs) < 1 {
		gorillalog.Warn("Unable to continue, no catalogs assigned: ", config.Current.Catalogs)
		os.Exit(1)
	}

	// Loop through the catalogs and get each one in order
	for _, catalog := range config.Current.Catalogs {

		catalogCount++

		// Download the catalog
		catalogURL := config.Current.URL + "catalogs/" + catalog + ".yaml"
		gorillalog.Info("Catalog Url:", catalogURL)
		err := downloadFile(config.CachePath, catalogURL)
		if err != nil {
			gorillalog.Error("Unable to retrieve catalog:", catalog, err)
		}

		// Open the catalog file
		yamlPath := filepath.Join(config.CachePath, catalog) + ".yaml"
		gorillalog.Debug("Catalog file path:", yamlPath)
		yamlFile, err := ioutil.ReadFile(yamlPath)
		if err != nil {
			gorillalog.Error("Unable to open the catalog file:", yamlPath, err)
		}

		// Parse the catalog
		var catalogItems map[string]Item
		err = yaml.Unmarshal(yamlFile, &catalogItems)
		if err != nil {
			gorillalog.Error("Unable to parse yaml catalog:", yamlPath, err)
		}

		// Add the new parsed catalog items to the catalogMap
		catalogMap[catalogCount] = catalogItems
	}

	return catalogMap
}
