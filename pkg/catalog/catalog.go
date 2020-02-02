package catalog

import (
	"fmt"
	"os"

	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/download"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	"github.com/1dustindavis/gorilla/pkg/report"
	"gopkg.in/yaml.v2"
)

// Item contains an individual entry from the catalog
type Item struct {
	Catalog      string        `yaml:"catalog",omitempty`
	Check        InstallCheck  `yaml:"check",omitempty`
	Dependencies []string      `yaml:"dependencies",omitempty`
	DisplayName  string        `yaml:"display_name",omitempty`
	Installer    InstallerItem `yaml:"installer",omitempty`
	Uninstaller  InstallerItem `yaml:"uninstaller",omitempty`
	Version      string        `yaml:"version",omitempty`
}

// InstallerItem holds information about how to install a catalog item
type InstallerItem struct {
	Type      string   `yaml:"type"`
	Location  string   `yaml:"location"`
	Hash      string   `yaml:"hash"`
	Arguments []string `yaml:"arguments",omitempty`
}

// InstallCheck holds information about how to check the status of a catalog item
type InstallCheck struct {
	File     []FileCheck `yaml:"file",omitempty`
	Script   string      `yaml:"script",omitempty`
	Registry RegCheck    `yaml:"registry",omitempty`
}

// FileCheck holds information about checking via a file
type FileCheck struct {
	Path        string `yaml:"path",omitempty`
	Version     string `yaml:"version",omitempty`
	ProductName string `yaml:"product_name",omitempty`
	Hash        string `yaml:"hash",omitempty`
}

// RegCheck holds information about checking via registry
type RegCheck struct {
	Name    string `yaml:"name",omitempty`
	Version string `yaml:"version",omitempty`
}

// This abstraction allows us to override the function while testing
var downloadGet = download.Get

// Get returns a map of `Item` from the catalog
func Get(cfg config.Configuration) map[int]map[string]Item {

	// catalogMap is an map of parsed catalogs
	var catalogMap = make(map[int]map[string]Item)

	// catalogCount allows us to be sure we are processing catalogs in order
	var catalogCount = 0

	// Setup to catch a potential failure
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
			report.End()
			os.Exit(1)

		}
	}()

	// Error if dont have at least one catalog
	if len(cfg.Catalogs) < 1 {
		gorillalog.Error("Unable to continue, no catalogs assigned: ", cfg.Catalogs)
	}

	// Loop through the catalogs and get each one in order
	for _, catalog := range cfg.Catalogs {

		catalogCount++

		// Download the catalog
		catalogURL := cfg.URL + "catalogs/" + catalog + ".yaml"
		gorillalog.Info("Catalog Url:", catalogURL)
		yamlFile, err := downloadGet(catalogURL)
		if err != nil {
			gorillalog.Error("Unable to retrieve catalog: ", err)
		}

		// Parse the catalog
		var catalogItems map[string]Item
		err = yaml.Unmarshal(yamlFile, &catalogItems)
		if err != nil {
			gorillalog.Error("Unable to parse yaml catalog: ", err)
		}

		// Add the new parsed catalog items to the catalogMap
		catalogMap[catalogCount] = catalogItems
	}

	return catalogMap
}
