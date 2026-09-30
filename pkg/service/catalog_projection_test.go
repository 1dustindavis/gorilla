package service

import (
	"testing"
	"time"

	"github.com/1dustindavis/gorilla/pkg/appcatalog"
	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/managedrun"
	"github.com/1dustindavis/gorilla/pkg/manifest"
	"github.com/1dustindavis/gorilla/pkg/status"
)

func TestPreparedOptionalProjectionDoesNotRetrieveRepository(t *testing.T) {
	originalManifestGet, originalCatalogGet, originalObserve := manifestGet, catalogGet, statusObserve
	t.Cleanup(func() {
		manifestGet, catalogGet, statusObserve = originalManifestGet, originalCatalogGet, originalObserve
	})
	manifestGet = func(config.Configuration) ([]manifest.Item, []string, error) {
		t.Fatal("prepared projection called manifest.Get")
		return nil, nil, nil
	}
	catalogGet = func(config.Configuration) (map[int]map[string]catalog.Item, error) {
		t.Fatal("prepared projection called catalog.Get")
		return nil, nil
	}

	checkedAt := time.Date(2026, 9, 29, 20, 0, 0, 0, time.UTC)
	observed := 0
	statusObserve = func(item catalog.Item, _, _ string) (status.Observation, error) {
		observed++
		return status.Observation{State: status.Installed, CheckedAtUTC: checkedAt}, nil
	}
	cfg := config.Configuration{AppDataPath: t.TempDir(), Catalogs: []string{"primary"}}
	prepared := managedrun.PreparedContext{
		Config:    cfg,
		Manifests: []manifest.Item{{OptionalInstalls: []string{"Example"}}},
		Catalogs: map[int]map[string]catalog.Item{1: {"Example": {
			DisplayName: "Example",
			Version:     "1.0",
			Installer:   catalog.InstallerItem{Type: "msi", Location: "example.msi"},
		}}},
	}

	details, err := getOptionalItemDetailsFromManagedContext(prepared)
	if err != nil {
		t.Fatal(err)
	}
	if observed != 1 {
		t.Fatalf("post-run observation count = %d, want 1", observed)
	}
	if len(details) != 1 || details[0].Contract.Observation.State != appcatalog.Installed {
		t.Fatalf("unexpected prepared projection: %+v", details)
	}
	if details[0].Contract.Observation.CheckedAtUTC == nil || !details[0].Contract.Observation.CheckedAtUTC.Equal(checkedAt) {
		t.Fatalf("fresh observation timestamp was not used: %+v", details[0].Contract.Observation)
	}
}
