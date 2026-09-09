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
	"github.com/1dustindavis/gorilla/pkg/manifest"
	"github.com/1dustindavis/gorilla/pkg/status"
)

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
		func(config.Configuration) error { return nil },
		func(_ config.Configuration, itemName, action string) (installer.Result, error) {
			gotItem, gotAction = itemName, action
			return installer.Result{ItemName: itemName, Action: "install", Outcome: installer.OutcomeSucceeded}, nil
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
	if terminal.State != "Succeeded" || terminal.Result == nil || terminal.Result.Outcome != appcatalog.Succeeded {
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
		func(config.Configuration) error { return nil },
		func(_ config.Configuration, itemName, _ string) (installer.Result, error) {
			if sr.execMutex.TryLock() {
				sr.execMutex.Unlock()
				t.Fatal("managed execution occurred outside execMutex")
			}
			return installer.Result{ItemName: itemName, Action: "install", Outcome: installer.OutcomeSucceeded}, nil
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

func TestExecuteManagedItemOperationFallsBackToLegacyManagedRun(t *testing.T) {
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
	sr := newServiceRunner(cfg, func(config.Configuration) error {
		called = true
		return nil
	})
	result, err := sr.executeManagedItemOperation(context.Background(), actionInstallItem, "Example", CommandResponse{})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("legacy managed run fallback was not called")
	}
	if result.Outcome != appcatalog.AlreadySatisfied {
		t.Fatalf("fallback full run should rely on verified no-op evidence, got %+v", result)
	}
}
