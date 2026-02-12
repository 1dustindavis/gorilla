package main

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/manifest"
)

func TestServiceLocalManifestAddRemoveList(t *testing.T) {
	cfg := config.Configuration{
		AppDataPath: filepath.Clean(t.TempDir()),
	}

	if err := addServiceManagedInstalls(cfg, []string{"GoogleChrome", "7zip"}); err != nil {
		t.Fatalf("addServiceManagedInstalls failed: %v", err)
	}
	if err := addServiceManagedInstalls(cfg, []string{"GoogleChrome"}); err != nil {
		t.Fatalf("addServiceManagedInstalls dedupe failed: %v", err)
	}

	items, err := listServiceManagedInstalls(cfg)
	if err != nil {
		t.Fatalf("listServiceManagedInstalls failed: %v", err)
	}
	if !reflect.DeepEqual(items, []string{"7zip", "GoogleChrome"}) {
		t.Fatalf("unexpected items after add: %#v", items)
	}

	if err := removeServiceManagedInstalls(cfg, []string{"GoogleChrome"}); err != nil {
		t.Fatalf("removeServiceManagedInstalls failed: %v", err)
	}

	items, err = listServiceManagedInstalls(cfg)
	if err != nil {
		t.Fatalf("listServiceManagedInstalls failed after remove: %v", err)
	}
	if !reflect.DeepEqual(items, []string{"7zip"}) {
		t.Fatalf("unexpected items after remove: %#v", items)
	}
}

func TestWithServiceLocalManifest(t *testing.T) {
	cfg := config.Configuration{
		AppDataPath: filepath.Clean(t.TempDir()),
	}

	cfg = withServiceLocalManifest(cfg)
	cfg = withServiceLocalManifest(cfg)

	if len(cfg.LocalManifests) != 1 {
		t.Fatalf("expected one local manifest entry, got %d", len(cfg.LocalManifests))
	}
	if cfg.LocalManifests[0] != serviceLocalManifestPath(cfg) {
		t.Fatalf("unexpected local manifest path: %s", cfg.LocalManifests[0])
	}
	if filepath.Base(cfg.LocalManifests[0]) != "service-manifest.yaml" {
		t.Fatalf("unexpected local manifest filename: %s", filepath.Base(cfg.LocalManifests[0]))
	}
}

func TestGetOptionalItems(t *testing.T) {
	origManifestGet := manifestGet
	defer func() { manifestGet = origManifestGet }()

	cfg := config.Configuration{
		AppDataPath: filepath.Clean(t.TempDir()),
	}
	if err := addServiceManagedInstalls(cfg, []string{"GoogleChrome"}); err != nil {
		t.Fatalf("addServiceManagedInstalls failed: %v", err)
	}

	manifestGet = func(_ config.Configuration) ([]manifest.Item, []string) {
		return []manifest.Item{
			{
				Name:             "base",
				OptionalInstalls: []string{"GoogleChrome", "7zip", "Firefox"},
			},
			{
				Name:             "extra",
				OptionalInstalls: []string{"7zip", "VSCode"},
			},
		}, nil
	}

	items, err := getOptionalItems(cfg)
	if err != nil {
		t.Fatalf("getOptionalItems failed: %v", err)
	}
	expected := []string{"7zip", "Firefox", "GoogleChrome", "VSCode"}
	if !reflect.DeepEqual(expected, items) {
		t.Fatalf("unexpected optional items, expected %#v, got %#v", expected, items)
	}
}
