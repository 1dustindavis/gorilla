//go:build windows

package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/1dustindavis/gorilla/pkg/appcatalog"
	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/installer"
	"github.com/1dustindavis/gorilla/pkg/managedrun"
	"github.com/1dustindavis/gorilla/pkg/manifest"
	"github.com/1dustindavis/gorilla/pkg/status"
)

func testAppCatalogItem(name string) appcatalog.Item {
	return appcatalog.Item{ItemName: name, DisplayName: name, Observation: appcatalog.Observation{State: appcatalog.Absent}}
}

func TestPersistentManagedRunPublishesFromReturnedPreparedContext(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir(), Catalogs: []string{"primary"}}
	prepared := managedrun.PreparedContext{
		Config:    cfg,
		Manifests: []manifest.Item{{OptionalInstalls: []string{"Example"}}},
		Catalogs: map[int]map[string]catalog.Item{1: {"Example": {
			DisplayName: "Example",
			Installer:   catalog.InstallerItem{Type: "msi", Location: "example.msi"},
		}}},
	}

	originalManifestGet, originalCatalogGet, originalObserve := manifestGet, catalogGet, statusObserve
	t.Cleanup(func() { manifestGet, catalogGet, statusObserve = originalManifestGet, originalCatalogGet, originalObserve })
	manifestGet = func(config.Configuration) ([]manifest.Item, []string, error) {
		t.Fatal("full-run snapshot re-fetched manifests")
		return nil, nil, nil
	}
	catalogGet = func(config.Configuration) (map[int]map[string]catalog.Item, error) {
		t.Fatal("full-run snapshot re-fetched catalogs")
		return nil, nil
	}
	managedCompleted := false
	statusObserve = func(catalog.Item, string, string) (status.Observation, error) {
		if !managedCompleted {
			t.Fatal("snapshot observation ran before managed execution completed")
		}
		return status.Observation{State: status.Absent, CheckedAtUTC: time.Now().UTC()}, nil
	}

	var sr *serviceRunner
	sr = newServiceRunner(cfg, func(config.Configuration) (managedrun.RunResult, error) {
		if sr.execMutex.TryLock() {
			sr.execMutex.Unlock()
			t.Fatal("managed run occurred outside execMutex")
		}
		managedCompleted = true
		return managedrun.RunResult{Prepared: prepared}, nil
	})

	// The queue normally owns this lock. Holding it explicitly proves the
	// snapshot is visible before the serialization boundary is released.
	sr.execMutex.Lock()
	resp, err := sr.executeCommandWithAdmission(Command{Action: actionRun})
	if err != nil {
		sr.execMutex.Unlock()
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		sr.execMutex.Unlock()
		t.Fatalf("unexpected run response: %+v", resp)
	}
	snapshot, ok := sr.currentCatalogSnapshot()
	if !ok || len(snapshot.Items) != 1 || snapshot.Items[0].ItemName != "Example" {
		sr.execMutex.Unlock()
		t.Fatalf("snapshot was not published inside serialization boundary: %#v", snapshot)
	}
	sr.execMutex.Unlock()
}

func TestManagedRunProjectionFailureRetainsPriorSnapshotAndSuccess(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir()}
	badAppData := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(badAppData, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	prepared := managedrun.PreparedContext{
		Config:    config.Configuration{AppDataPath: badAppData},
		Manifests: []manifest.Item{{OptionalInstalls: []string{"Example"}}},
	}
	sr := newServiceRunner(cfg, func(config.Configuration) (managedrun.RunResult, error) {
		return managedrun.RunResult{Prepared: prepared}, nil
	})
	sr.catalogSnapshot = testSnapshot(t, cfg, "Previous")

	resp, err := sr.executeManagedRunCommand(Command{Action: actionRun})
	if err != nil {
		t.Fatalf("snapshot projection failure changed successful managed run into failure: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("unexpected response after projection failure: %+v", resp)
	}
	snapshot, _ := sr.currentCatalogSnapshot()
	if len(snapshot.Items) != 1 || snapshot.Items[0].ItemName != "Previous" {
		t.Fatalf("projection failure replaced last-known-good snapshot: %#v", snapshot)
	}
}

func TestTargetedInstallUsesExecutionPreparedWithoutPostExecutionRepositoryFetch(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir(), Catalogs: []string{"primary"}}
	if err := addServiceManagedInstalls(cfg, []string{"Example"}); err != nil {
		t.Fatal(err)
	}
	prepared := managedrun.PreparedContext{
		Config:    cfg,
		Manifests: []manifest.Item{{OptionalInstalls: []string{"Example"}}},
		Catalogs: map[int]map[string]catalog.Item{1: {"Example": {
			DisplayName: "Example",
			Installer:   catalog.InstallerItem{Type: "msi", Location: "example.msi"},
		}}},
	}

	originalManifestGet, originalCatalogGet, originalObserve := manifestGet, catalogGet, statusObserve
	t.Cleanup(func() { manifestGet, catalogGet, statusObserve = originalManifestGet, originalCatalogGet, originalObserve })
	executed := false
	manifestGet = func(config.Configuration) ([]manifest.Item, []string, error) {
		if executed {
			t.Fatal("targeted Install re-fetched manifests after execution")
		}
		return []manifest.Item{}, nil, nil
	}
	catalogGet = func(config.Configuration) (map[int]map[string]catalog.Item, error) {
		t.Fatal("targeted Install re-fetched catalogs")
		return nil, nil
	}
	statusObserve = func(catalog.Item, string, string) (status.Observation, error) {
		if !executed {
			t.Fatal("post-operation observation occurred before execution")
		}
		return status.Observation{State: status.Installed, ActionNeeded: false, CheckedAtUTC: time.Now().UTC()}, nil
	}

	sr := newServiceRunner(
		cfg,
		func(config.Configuration) (managedrun.RunResult, error) { return managedrun.RunResult{}, nil },
		func(_ config.Configuration, itemName, action string) (managedrun.ItemRunResult, error) {
			executed = true
			return managedrun.ItemRunResult{
				ExecutionPrepared: prepared,
				Execution: installer.Result{ItemName: itemName, Action: action, Outcome: installer.OutcomeSucceeded},
			}, nil
		},
	)
	result, err := sr.executeManagedItemOperation(context.Background(), actionInstallItem, "Example", CommandResponse{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != appcatalog.Succeeded {
		t.Fatalf("unexpected targeted Install verification: %+v", result)
	}
	snapshot, ok := sr.currentCatalogSnapshot()
	if !ok || len(snapshot.Items) != 1 || snapshot.Items[0].ItemName != "Example" {
		t.Fatalf("targeted Install did not publish projected snapshot: %#v", snapshot)
	}
}
