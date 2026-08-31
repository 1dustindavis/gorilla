//go:build windows

package service

import (
	"errors"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
)

func TestBuildOptionalInstallResponseItemsReturnsAuthoritativeStatusAndMetadata(t *testing.T) {
	originalCatalogGet := optionalInstallCatalogGet
	originalStatusCheck := optionalInstallStatusCheck
	t.Cleanup(func() {
		optionalInstallCatalogGet = originalCatalogGet
		optionalInstallStatusCheck = originalStatusCheck
	})

	cfg := config.Configuration{Catalogs: []string{"apps"}, CachePath: `C:\cache`}
	item := catalog.Item{
		DisplayName: "Fixture App",
		Version:     "1.2.3",
		Installer: catalog.InstallerItem{
			Type:      "ps1",
			PackageID: "fixture-package",
			Location:  "scripts/fixture.ps1",
		},
	}
	optionalInstallCatalogGet = func(got config.Configuration) (map[int]map[string]catalog.Item, error) {
		return map[int]map[string]catalog.Item{1: {"Fixture": item}}, nil
	}

	installed := false
	optionalInstallStatusCheck = func(got catalog.Item, action, cachePath string) (bool, error) {
		if got.DisplayName != item.DisplayName || action != "install" || cachePath != cfg.CachePath {
			t.Fatalf("unexpected status check: item=%#v action=%q cachePath=%q", got, action, cachePath)
		}
		return !installed, nil
	}

	items, err := buildOptionalInstallResponseItems(cfg, []string{"Fixture"})
	if err != nil {
		t.Fatalf("build optional items: %v", err)
	}
	assertOptionalInstallResponse(t, items[0], false, "NotInstalled")

	installed = true
	items, err = buildOptionalInstallResponseItems(cfg, []string{"Fixture"})
	if err != nil {
		t.Fatalf("build optional items after install: %v", err)
	}
	assertOptionalInstallResponse(t, items[0], true, "Installed")
}

func TestBuildOptionalInstallResponseItemsKeepsUnknownWhenStatusCheckFails(t *testing.T) {
	originalCatalogGet := optionalInstallCatalogGet
	originalStatusCheck := optionalInstallStatusCheck
	t.Cleanup(func() {
		optionalInstallCatalogGet = originalCatalogGet
		optionalInstallStatusCheck = originalStatusCheck
	})

	cfg := config.Configuration{Catalogs: []string{"apps"}}
	optionalInstallCatalogGet = func(config.Configuration) (map[int]map[string]catalog.Item, error) {
		return map[int]map[string]catalog.Item{1: {"Fixture": {DisplayName: "Fixture"}}}, nil
	}
	optionalInstallStatusCheck = func(catalog.Item, string, string) (bool, error) {
		return false, errors.New("status unavailable")
	}

	items, err := buildOptionalInstallResponseItems(cfg, []string{"Fixture"})
	if err != nil {
		t.Fatalf("build optional items: %v", err)
	}
	if items[0].IsInstalled || items[0].Status != "Unknown" {
		t.Fatalf("expected unknown status, got installed=%v status=%q", items[0].IsInstalled, items[0].Status)
	}
}

func TestBuildOptionalInstallResponseItemsFailsWhenCatalogLoadFails(t *testing.T) {
	originalCatalogGet := optionalInstallCatalogGet
	t.Cleanup(func() { optionalInstallCatalogGet = originalCatalogGet })

	optionalInstallCatalogGet = func(config.Configuration) (map[int]map[string]catalog.Item, error) {
		return nil, errors.New("catalog unavailable")
	}

	if _, err := buildOptionalInstallResponseItems(config.Configuration{}, []string{"Fixture"}); err == nil {
		t.Fatal("expected catalog load error")
	}
}

func assertOptionalInstallResponse(t *testing.T, got optionalInstallResponseItem, installed bool, status string) {
	t.Helper()
	if got.ItemName != "Fixture" || got.DisplayName != "Fixture App" {
		t.Fatalf("unexpected identity: %#v", got)
	}
	if got.Version != "1.2.3" || got.Catalog != "apps" {
		t.Fatalf("unexpected catalog metadata: %#v", got)
	}
	if got.InstallerType != "ps1" || got.InstallerPackageID != "fixture-package" || got.InstallerLocation != "scripts/fixture.ps1" {
		t.Fatalf("unexpected installer metadata: %#v", got)
	}
	if !got.IsManaged || got.IsInstalled != installed || got.Status != status {
		t.Fatalf("unexpected state: %#v", got)
	}
	if got.StatusUpdatedAtUTC == "" {
		t.Fatal("expected status timestamp")
	}
}
