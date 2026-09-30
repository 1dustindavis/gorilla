package service

import (
	"testing"
	"time"

	"github.com/1dustindavis/gorilla/pkg/appcatalog"
)

func TestOptionalInstallResponseItemsPreservesLegacyAndStructuredFields(t *testing.T) {
	generatedAt := time.Date(2026, 9, 29, 21, 0, 0, 0, time.UTC)
	target := "2.0"
	details := []optionalItemDetails{{
		Contract: appcatalog.Item{
			ItemName:      "Example",
			DisplayName:   "Example App",
			Description:   "Example description",
			IconPath:      `C:\ProgramData\gorilla\icons\example.png`,
			Catalog:       "primary",
			TargetVersion: &target,
			Observation: appcatalog.Observation{
				State:              appcatalog.UpdateAvailable,
				InstalledVersion:   stringPtr("1.0"),
				InstallRequirement: appcatalog.RequirementNotSatisfied,
			},
			Policy: appcatalog.Policy{Optional: true, Selection: appcatalog.KeepInstalled},
			Actions: appcatalog.Actions{
				Install: appcatalog.ActionDecision{Allowed: true},
			},
		},
		InstallerType:     "msi",
		InstallerLocation: "example.msi",
	}}

	items := optionalInstallResponseItems(details, generatedAt)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	item := items[0]
	if item.ItemName != "Example" || item.DisplayName != "Example App" || item.Description != "Example description" || item.IconPath == "" {
		t.Fatalf("presentation fields drifted: %+v", item)
	}
	if item.Version != "2.0" || item.TargetVersion == nil || *item.TargetVersion != "2.0" {
		t.Fatalf("target/version mapping drifted: %+v", item)
	}
	if item.InstallerPackageID != "Example" {
		t.Fatalf("package ID fallback = %q, want item name", item.InstallerPackageID)
	}
	if !item.IsManaged || !item.IsInstalled || item.Status != "UpdateAvailable" {
		t.Fatalf("legacy state mapping drifted: %+v", item)
	}
	if item.StatusUpdatedAtUTC != generatedAt.Format(time.RFC3339) {
		t.Fatalf("fallback timestamp = %q, want %q", item.StatusUpdatedAtUTC, generatedAt.Format(time.RFC3339))
	}
	if item.Observation.State != appcatalog.UpdateAvailable || !item.Policy.Optional || !item.Actions.Install.Allowed {
		t.Fatalf("structured fields drifted: %+v", item)
	}
}

func TestOptionalInstallResponseItemsUsesObservationTimestamp(t *testing.T) {
	generatedAt := time.Date(2026, 9, 29, 21, 0, 0, 0, time.UTC)
	checkedAt := generatedAt.Add(-5 * time.Minute)
	items := optionalInstallResponseItems([]optionalItemDetails{{
		Contract: appcatalog.Item{
			ItemName: "Example",
			Observation: appcatalog.Observation{
				State:        appcatalog.Absent,
				CheckedAtUTC: &checkedAt,
			},
		},
	}}, generatedAt)
	if items[0].StatusUpdatedAtUTC != checkedAt.Format(time.RFC3339) {
		t.Fatalf("observation timestamp = %q, want %q", items[0].StatusUpdatedAtUTC, checkedAt.Format(time.RFC3339))
	}
	if items[0].Status != "NotInstalled" || items[0].IsInstalled {
		t.Fatalf("absent legacy mapping drifted: %+v", items[0])
	}
}

func stringPtr(value string) *string { return &value }
