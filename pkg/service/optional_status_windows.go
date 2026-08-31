//go:build windows

package service

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/status"
)

const resolvedOptionalInstallPrefix = "gorilla-resolved-optional-item:"

var (
	optionalInstallCatalogGet  = catalog.Get
	optionalInstallStatusCheck = status.CheckStatus
)

func buildOptionalInstallResponseItems(cfg config.Configuration, names []string) ([]optionalInstallResponseItem, error) {
	if resolved, ok, err := decodeResolvedOptionalInstallResponseItems(names); ok || err != nil {
		return resolved, err
	}

	catalogs, err := optionalInstallCatalogGet(cfg)
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

		catalogItem, catalogName, ok := findOptionalInstallCatalogItem(cfg, catalogs, name)
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
		if response.InstallerPackageID == "" {
			response.InstallerPackageID = name
		}
		response.InstallerLocation = catalogItem.Installer.Location

		if hasOptionalInstallStatusCheck(catalogItem) {
			actionNeeded, checkErr := optionalInstallStatusCheck(catalogItem, "install", cfg.CachePath)
			if checkErr == nil {
				response.IsInstalled = !actionNeeded
				if response.IsInstalled {
					response.Status = "Installed"
				} else {
					response.Status = "NotInstalled"
				}
			}
		}
		items = append(items, response)
	}
	return items, nil
}

func encodeResolvedOptionalInstallResponseItems(items []optionalInstallResponseItem) ([]string, error) {
	encoded := make([]string, 0, len(items))
	for _, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			return nil, fmt.Errorf("encode resolved optional install item %q: %w", item.ItemName, err)
		}
		encoded = append(encoded, resolvedOptionalInstallPrefix+string(data))
	}
	return encoded, nil
}

func decodeResolvedOptionalInstallResponseItems(values []string) ([]optionalInstallResponseItem, bool, error) {
	if len(values) == 0 || !strings.HasPrefix(values[0], resolvedOptionalInstallPrefix) {
		return nil, false, nil
	}
	items := make([]optionalInstallResponseItem, 0, len(values))
	for _, value := range values {
		if !strings.HasPrefix(value, resolvedOptionalInstallPrefix) {
			return nil, true, errorsNewMixedResolvedOptionalItems()
		}
		var item optionalInstallResponseItem
		if err := json.Unmarshal([]byte(strings.TrimPrefix(value, resolvedOptionalInstallPrefix)), &item); err != nil {
			return nil, true, fmt.Errorf("decode resolved optional install item: %w", err)
		}
		items = append(items, item)
	}
	return items, true, nil
}

func errorsNewMixedResolvedOptionalItems() error {
	return fmt.Errorf("resolved optional install response contains mixed item encodings")
}

func hasOptionalInstallStatusCheck(item catalog.Item) bool {
	return item.Check.Script != "" ||
		item.Check.File != nil ||
		item.Check.Registry.Version != "" ||
		item.Check.Appx.Name != ""
}

func findOptionalInstallCatalogItem(
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
