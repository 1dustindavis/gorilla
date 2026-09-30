package service

import (
	"time"

	"github.com/1dustindavis/gorilla/pkg/appcatalog"
)

func optionalInstallResponseItems(details []optionalItemDetails, generatedAt time.Time) []optionalInstallResponseItem {
	items := make([]optionalInstallResponseItem, 0, len(details))
	fallbackTimestamp := generatedAt.UTC().Format(time.RFC3339)
	for _, detail := range details {
		item := detail.Contract
		version := ""
		if item.TargetVersion != nil {
			version = *item.TargetVersion
		}
		updated := ""
		if item.Observation.CheckedAtUTC != nil {
			updated = item.Observation.CheckedAtUTC.Format(time.RFC3339)
		}
		if updated == "" {
			updated = fallbackTimestamp
		}
		installed := item.Observation.State == appcatalog.Installed || item.Observation.State == appcatalog.UpdateAvailable
		legacyStatus := "Unknown"
		switch item.Observation.State {
		case appcatalog.Absent:
			legacyStatus = "NotInstalled"
		case appcatalog.Installed:
			legacyStatus = "Installed"
		case appcatalog.UpdateAvailable:
			legacyStatus = "UpdateAvailable"
		}
		packageID := detail.InstallerPackageID
		if packageID == "" {
			packageID = item.ItemName
		}
		items = append(items, optionalInstallResponseItem{
			ItemName:           item.ItemName,
			DisplayName:        item.DisplayName,
			Description:        item.Description,
			IconPath:           item.IconPath,
			Version:            version,
			Catalog:            item.Catalog,
			InstallerType:      detail.InstallerType,
			InstallerPackageID: packageID,
			InstallerLocation:  detail.InstallerLocation,
			IsManaged:          item.Policy.Selection == appcatalog.KeepInstalled,
			IsInstalled:        installed,
			Status:             legacyStatus,
			StatusUpdatedAtUTC: updated,
			TargetVersion:      item.TargetVersion,
			Observation:        item.Observation,
			Policy:             item.Policy,
			Actions:            item.Actions,
		})
	}
	return items
}
