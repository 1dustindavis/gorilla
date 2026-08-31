//go:build windows

package service

import (
	"fmt"
	"slices"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/status"
)

var optionalInstallStatusCheck = status.CheckStatus

func buildOptionalInstallResponseItems(cfg config.Configuration, names []string) ([]optionalInstallResponseItem, error) {
	catalogs, err := catalog.Get(cfg)
	if err != nil {
		return nil, fmt.Errorf("load catalogs for optional install status: %w", err)
	}

	sorted := append([]string(nil), names...)
	slices.Sort(sorted)
	items := make([]optionalInstallResponseItem, 0, len(sorted))
	for _, name := range sorted {
		response := optionalInstallResponseItem{
			ItemName:           name,
			DisplayName:        name,
			InstallerPackageID: name,
			IsManaged:          true,
			Status:             "Unknown",
			StatusUpdatedAtUTC: nowRFC3339UTC(),
		}

		catalogItem, catalogName, ok := findCatalogItem(cfg, catalogs, name)
		if !ok {
			items = append(items, response)
			continue
		}

		response.DisplayName = catalogItem.DisplayName
		if response.DisplayName == "" {
			response.DisplayName = name
		}
		response.Version = catalogItem.Version
		response.Catalog = catalogName
		response.InstallerType = catalogItem.Installer.Type
		response.InstallerPackageID = catalogItem.Installer.PackageID
		response.InstallerLocation = catalogItem.Installer.Location

		actionNeeded, checkErr := optionalInstallStatusCheck(catalogItem, "install", cfg.CachePath)
		if checkErr == nil {
			response.IsInstalled = !actionNeeded
			if response.IsInstalled {
				response.Status = "Installed"
			} else {
				response.Status = "NotInstalled"
			}
		}
		items = append(items, response)
	}
	return items, nil
}

func findCatalogItem(
	cfg config.Configuration,
	catalogs map[int]map[string]catalog.Item,
	name string,
) (catalog.Item, string, bool) {
	for index, catalogName := range cfg.Catalogs {
		catalogItems := catalogs[index+1]
		if item, ok := catalogItems[name]; ok {
			return item, catalogName, true
		}
	}
	return catalog.Item{}, "", false
}
