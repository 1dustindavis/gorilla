package catalog

import (
	"io/ioutil"
	"path/filepath"

	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/download"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	"gopkg.in/yaml.v2"
)

// Item contains an individual entry from the catalog
type Item struct {
	Dependencies           []string `yaml:"dependencies"`
	DisplayName            string   `yaml:"display_name"`
	InstallCheckPath       string   `yaml:"install_check_path"`
	InstallCheckPathHash   string   `yaml:"install_check_path_hash"`
	InstallCheckScript     string   `yaml:"install_check_script"`
	InstallerItemArguments []string `yaml:"installer_item_arguments"`
	InstallerItemHash      string   `yaml:"installer_item_hash"`
	InstallerItemLocation  string   `yaml:"installer_item_location"`
	InstallerType          string   `yaml:"installer_type"`
	UninstallMethod        string   `yaml:"uninstall_method"`
	Version                string   `yaml:"version"`
}

var downloadFile = download.File

// Get returns a map of `Item` from the catalog
func Get() map[string]Item {

	// Download the catalog
	catalogURL := config.URL + "catalogs/" + config.Catalog + ".yaml"
	gorillalog.Info("Catalog Url:", catalogURL)
	err := downloadFile(config.CachePath, catalogURL)
	if err != nil {
		gorillalog.Error("Unable to retrieve catalog:", config.Catalog, err)
	}

	// Parse the catalog
	yamlPath := filepath.Join(config.CachePath, config.Catalog) + ".yaml"
	gorillalog.Debug("Catalog file path:", yamlPath)
	yamlFile, err := ioutil.ReadFile(yamlPath)
	var catalog map[string]Item
	err = yaml.Unmarshal(yamlFile, &catalog)
	if err != nil {
		gorillalog.Error("Unable to parse yaml catalog:", yamlPath, err)
	}
	return catalog
}
