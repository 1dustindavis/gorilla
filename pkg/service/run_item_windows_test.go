//go:build windows

package service

import (
	"context"
	"testing"
	"time"

	"github.com/1dustindavis/gorilla/pkg/appcatalog"
	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/installer"
	managed "github.com/1dustindavis/gorilla/pkg/managedrun"
	"github.com/1dustindavis/gorilla/pkg/manifest"
	"github.com/1dustindavis/gorilla/pkg/status"
)

func preparedInstallContext(cfg config.Configuration) managed.PreparedContext {
	cfg.Catalogs = []string{"primary"}
	return managed.PreparedContext{
		Config:    cfg,
		Manifests: []manifest.Item{{OptionalInstalls: []string{"Example"}}},
		Catalogs: map[int]map[string]catalog.Item{1: {"Example": {
			DisplayName: "Example",
			Installer:   catalog.InstallerItem{Type: "msi", Location: "example.msi"},
		}}},
	}
}

func TestScheduleRunAfterMutationCarriesVerifiedRequestedItemResult(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir()}
	stubOptionalCatalog(t,
		[]manifest.Item{{OptionalInstalls: []string{"Example"}}},
		map[int]map[string]catalog.Item{1: {"Example": {
			DisplayName: "Example",
			Installer:   catalog.InstallerItem{Type: "msi", Location: "example.msi"},
		}}},
		map[string]status.Observation{"Example": {
			State:        status.Installed,
			ActionNeeded: false,
			CheckedAtUTC: time.Now().UTC(),
		}},
	)
	if err := addServiceManagedInstalls(cfg, []string{"Example"}); err != nil {
		t.Fatal(err)
	}

	var gotItem, gotAction string
	sr := newServiceRunner(
		cfg,
		func(config.Configuration) (managed.RunResult, error) { return managed.RunResult{}, nil },
		func(_ config.Configuration, itemName, action string) (managed.ItemRunResult, error) {
			gotItem, gotAction = itemName, action
			return managed.ItemRunResult{
				ExecutionPrepared: preparedInstallContext(cfg),
				Execution: installer.Result{
					ItemName: itemName,
					Action:   "install",
					Outcome:  installer.OutcomeSucceeded,
				},
			}, nil
		},
	)
	operationID := "verified-op"
	sr.registerTrackedOperation(operationID, "Example", actionInstallItem)
	sr.scheduleRunAfterMutation(context.Background(), actionInstallItem, CommandResponse{OperationID: operationID}, "Example")
	sr.wg.Wait()

	if gotItem != "Example" || gotAction != actionInstallItem {
		t.Fatalf("managed item identity was not preserved: item=%q action=%q", gotItem, gotAction)
	}
	events, done, ok := sr.snapshotTrackedOperation(operationID)
	if !ok || !done {
		t.Fatalf("expected completed tracked operation, ok=%v done=%v", ok, done)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want queued/running/terminal", len(events))
	}
	for _, event := range events {
		if event.ProgressPercent != 0 {
			t.Fatalf("unexpected synthetic progress in event: %+v", event)
		}
		if event.ItemName != "Example" || event.Action != appcatalog.InstallAction {
			t.Fatalf("operation identity changed across events: %+v", event)
		}
	}
	terminal := events[len(events)-1]
	if terminal.State != "Completed" || terminal.Result == nil || terminal.Result.Outcome != appcatalog.Succeeded {
		t.Fatalf("unexpected terminal result: %+v", terminal)
	}
}

func TestExecuteManagedItemOperationSerializesVerificationWithExecution(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir()}
	stubOptionalCatalog(t,
		[]manifest.Item{{OptionalInstalls: []string{"Example"}}},
		map[int]map[string]catalog.Item{1: {"Example": {
			DisplayName: "Example",
			Installer:   catalog.InstallerItem{Type: "msi", Location: "example.msi"},
		}}},
		map[string]status.Observation{"Example": {
			State:        status.Installed,
			ActionNeeded: false,
			CheckedAtUTC: time.Now().UTC(),
		}},
	)
	if err := addServiceManagedInstalls(cfg, []string{"Example"}); err != nil {
		t.Fatal(err)
	}

	var sr *serviceRunner
	sr = newServiceRunner(
		cfg,
		func(config.Configuration) (managed.RunResult, error) { return managed.RunResult{}, nil },
		func(_ config.Configuration, itemName, _ string) (managed.ItemRunResult, error) {
			if sr.execMutex.TryLock() {
				sr.execMutex.Unlock()
				t.Fatal("managed execution occurred outside execMutex")
			}
			return managed.ItemRunResult{
				ExecutionPrepared: preparedInstallContext(cfg),
				Execution:         installer.Result{ItemName: itemName, Action: "install", Outcome: installer.OutcomeSucceeded},
			}, nil
		},
	)

	observed := statusObserve
	statusObserve = func(item catalog.Item, installType, cachePath string) (status.Observation, error) {
		if sr.execMutex.TryLock() {
			sr.execMutex.Unlock()
			t.Fatal("postcondition observation occurred outside execMutex")
		}
		return observed(item, installType, cachePath)
	}
	defer func() { statusObserve = observed }()

	result, err := sr.executeManagedItemOperation(context.Background(), actionInstallItem, "Example", CommandResponse{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != appcatalog.Succeeded {
		t.Fatalf("unexpected verified result: %+v", result)
	}
}

func TestExecuteManagedItemRemoveUsesTransientExecutionContextButPersistentVerification(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir()}
	stubOptionalCatalog(t,
		[]manifest.Item{{OptionalInstalls: []string{"Example"}}},
		map[int]map[string]catalog.Item{1: {"Example": {
			DisplayName: "Example",
			Installer:   catalog.InstallerItem{Type: "msi", Location: "example.msi"},
			Uninstaller: catalog.InstallerItem{Type: "msi", Location: "example.msi"},
		}}},
		map[string]status.Observation{"Example": {
			State:        status.Absent,
			ActionNeeded: true,
			CheckedAtUTC: time.Now().UTC(),
		}},
	)
	if err := addServiceManagedInstalls(cfg, []string{"Example"}); err != nil {
		t.Fatal(err)
	}

	runCfg, cleanupPath, err := prepareOneTimeRemoval(cfg, "Example", "transient-remove", true)
	if err != nil {
		t.Fatal(err)
	}
	if cleanupPath == "" || len(runCfg.LocalManifests) != len(cfg.LocalManifests)+1 {
		t.Fatalf("expected one-time removal manifest in execution config: %+v", runCfg.LocalManifests)
	}
	transientManifest := runCfg.LocalManifests[len(runCfg.LocalManifests)-1]

	baseManifestGet := manifestGet
	manifestGet = func(got config.Configuration) ([]manifest.Item, []string, error) {
		for _, path := range got.LocalManifests {
			if path == transientManifest {
				t.Fatalf("postcondition verification used transient removal manifest %q", transientManifest)
			}
		}
		return baseManifestGet(got)
	}

	var executionCfg config.Configuration
	sr := newServiceRunner(
		cfg,
		func(config.Configuration) (managed.RunResult, error) { return managed.RunResult{}, nil },
		func(got config.Configuration, itemName, action string) (managed.ItemRunResult, error) {
			executionCfg = got
			return managed.ItemRunResult{
				ExecutionPrepared: managed.PreparedContext{
					Config: got,
					Manifests: []manifest.Item{{OptionalInstalls: []string{"Example"}, Uninstalls: []string{"Example"}}},
				},
				Execution: installer.Result{
					ItemName: itemName,
					Action:   "uninstall",
					Outcome:  installer.OutcomeSucceeded,
				},
			}, nil
		},
	)

	result, err := sr.executeManagedItemOperation(
		context.Background(),
		actionRemoveItem,
		"Example",
		CommandResponse{RunConfig: &runCfg, CleanupPath: cleanupPath},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(executionCfg.LocalManifests) == 0 || executionCfg.LocalManifests[len(executionCfg.LocalManifests)-1] != transientManifest {
		t.Fatalf("targeted remove did not receive transient execution config: %+v", executionCfg.LocalManifests)
	}
	if result.Outcome != appcatalog.Succeeded {
		t.Fatalf("unexpected verified remove result: %+v", result)
	}
	snapshot, ok := sr.currentCatalogSnapshot()
	if !ok || len(snapshot.Items) != 1 {
		t.Fatalf("expected persistent snapshot after remove, got %#v", snapshot)
	}
	if snapshot.Items[0].Policy.RequiredUninstall {
		t.Fatalf("transient removal policy leaked into snapshot: %+v", snapshot.Items[0].Policy)
	}
}

func TestExecuteManagedItemOperationFallsBackToManagedRun(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir()}
	stubOptionalCatalog(t,
		[]manifest.Item{{OptionalInstalls: []string{"Example"}}},
		map[int]map[string]catalog.Item{1: {"Example": {
			DisplayName: "Example",
			Installer:   catalog.InstallerItem{Type: "msi", Location: "example.msi"},
		}}},
		map[string]status.Observation{"Example": {
			State:        status.Installed,
			ActionNeeded: false,
			CheckedAtUTC: time.Now().UTC(),
		}},
	)
	if err := addServiceManagedInstalls(cfg, []string{"Example"}); err != nil {
		t.Fatal(err)
	}

	called := false
	sr := newServiceRunner(cfg, func(config.Configuration) (managed.RunResult, error) {
		called = true
		return managed.RunResult{Prepared: preparedInstallContext(cfg)}, nil
	})
	result, err := sr.executeManagedItemOperation(context.Background(), actionInstallItem, "Example", CommandResponse{})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("managed run fallback was not called")
	}
	if result.Outcome != appcatalog.AlreadySatisfied {
		t.Fatalf("fallback full run should rely on verified no-op evidence, got %+v", result)
	}
}
