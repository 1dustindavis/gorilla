package service

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/manifest"
	"github.com/1dustindavis/gorilla/pkg/status"
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

func TestExecuteCommandInstallWritesManifestAndDoesNotRunInline(t *testing.T) {
	originalManifestGet, originalCatalogGet, originalObserve := manifestGet, catalogGet, statusObserve
	t.Cleanup(func() {
		manifestGet, catalogGet, statusObserve = originalManifestGet, originalCatalogGet, originalObserve
	})
	cfg := config.Configuration{
		AppDataPath:    filepath.Clean(t.TempDir()),
		LocalManifests: []string{"already-local.yaml"},
	}
	manifestGet = func(config.Configuration) ([]manifest.Item, []string, error) {
		return []manifest.Item{{OptionalInstalls: []string{"GoogleChrome"}}}, nil, nil
	}
	catalogGet = func(config.Configuration) (map[int]map[string]catalog.Item, error) {
		return map[int]map[string]catalog.Item{1: {"GoogleChrome": {
			DisplayName: "Google Chrome", Installer: catalog.InstallerItem{Type: "msi", Location: "chrome.msi"},
		}}}, nil
	}
	statusObserve = func(catalog.Item, string, string) (status.Observation, error) {
		return status.Observation{State: status.Absent}, nil
	}

	managedRunCalled := false
	managedRun := func(in config.Configuration) error {
		managedRunCalled = true
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

	if managedRunCalled {
		t.Fatalf("expected managed run to be deferred, but it ran inline")
	}
}
