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
