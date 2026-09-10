//go:build windows

package service

import (
	"testing"

	"github.com/1dustindavis/gorilla/pkg/config"
)

func TestSnapshotTrackedOperationsReturnsLatestRetainedState(t *testing.T) {
	sr := newServiceRunner(config.Configuration{}, func(config.Configuration) error { return nil })
	sr.registerTrackedOperation("op-1", "Slack", actionInstallItem)
	sr.appendOperationEvent("op-1", operationStatusEventPayload{
		State:    "Installing",
		Message:  "Installing item via managed run",
		ItemName: "Slack",
		Action:   "Install",
	})

	operations := sr.snapshotTrackedOperations()
	if len(operations) != 1 {
		t.Fatalf("expected one operation snapshot, got %d", len(operations))
	}
	if operations[0].OperationID != "op-1" {
		t.Fatalf("expected operation op-1, got %q", operations[0].OperationID)
	}
	if operations[0].Status.State != "Installing" {
		t.Fatalf("expected latest state Installing, got %q", operations[0].Status.State)
	}
	if operations[0].Status.ItemName != "Slack" || operations[0].Status.Action != "Install" {
		t.Fatalf("unexpected operation identity: %+v", operations[0].Status)
	}
}

func TestNewServiceRunnerDoesNotRecoverPreviousProcessOperations(t *testing.T) {
	first := newServiceRunner(config.Configuration{}, func(config.Configuration) error { return nil })
	first.registerTrackedOperation("op-1", "Slack", actionInstallItem)
	if len(first.snapshotTrackedOperations()) != 1 {
		t.Fatal("expected first process to retain its operation")
	}

	restarted := newServiceRunner(config.Configuration{}, func(config.Configuration) error { return nil })
	if got := restarted.snapshotTrackedOperations(); len(got) != 0 {
		t.Fatalf("expected restarted service to begin with empty operation registry, got %+v", got)
	}
}
