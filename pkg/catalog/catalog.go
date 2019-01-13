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
	Dependencies             []string `yaml:"dependencies"`
	DisplayName              string   `yaml:"display_name"`
	InstallCheckPath         string   `yaml:"install_check_path"`
	InstallCheckPathHash     string   `yaml:"install_check_path_hash"`
	InstallCheckScript       string   `yaml:"install_check_script"`
	InstallerItemArguments   []string `yaml:"installer_item_arguments"`
	InstallerItemHash        string   `yaml:"installer_item_hash"`
	InstallerItemLocation    string   `yaml:"installer_item_location"`
	InstallerType            string   `yaml:"installer_type"`
	UninstallerItemArguments []string `yaml:"uninstaller_item_arguments"`
	UninstallerItemHash      string   `yaml:"uninstaller_item_hash"`
	UninstallerItemLocation  string   `yaml:"uninstaller_item_location"`
	UninstallerType          string   `yaml:"uninstaller_type"`
	Version                  string   `yaml:"version"`
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
