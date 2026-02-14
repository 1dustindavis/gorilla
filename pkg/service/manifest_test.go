package service

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

func TestExecuteCommandRunPassesCfgThrough(t *testing.T) {
	cfg := config.Configuration{
		AppDataPath:    filepath.Clean(t.TempDir()),
		LocalManifests: []string{"already-local.yaml"},
	}

	var gotCfg config.Configuration
	managedRun := func(in config.Configuration) error {
		gotCfg = in
		return nil
	}

	resp, err := executeCommand(cfg, Command{Action: actionRun}, managedRun)
	if err != nil {
		t.Fatalf("executeCommand(run) failed: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %q", resp.Status)
	}
	if !reflect.DeepEqual(gotCfg.LocalManifests, cfg.LocalManifests) {
		t.Fatalf("expected managed run cfg local manifests %#v, got %#v", cfg.LocalManifests, gotCfg.LocalManifests)
	}
}

func TestExecuteCommandInstallWritesManifestAndRuns(t *testing.T) {
	cfg := config.Configuration{
		AppDataPath:    filepath.Clean(t.TempDir()),
		LocalManifests: []string{"already-local.yaml"},
	}

	var gotCfg config.Configuration
	managedRun := func(in config.Configuration) error {
		gotCfg = in
		return nil
	}

	resp, err := executeCommand(cfg, Command{Action: actionInstallItem, Items: []string{"GoogleChrome"}}, managedRun)
	if err != nil {
		t.Fatalf("executeCommand(install) failed: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %q", resp.Status)
	}

	items, err := listServiceManagedInstalls(cfg)
	if err != nil {
		t.Fatalf("listServiceManagedInstalls failed: %v", err)
	}
	if !reflect.DeepEqual(items, []string{"GoogleChrome"}) {
		t.Fatalf("unexpected service-manifest items: %#v", items)
	}

	if !reflect.DeepEqual(gotCfg.LocalManifests, cfg.LocalManifests) {
		t.Fatalf("expected managed run cfg local manifests %#v, got %#v", cfg.LocalManifests, gotCfg.LocalManifests)
	}
}
