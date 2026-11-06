//go:build windows

package service

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/1dustindavis/gorilla/pkg/config"
	managed "github.com/1dustindavis/gorilla/pkg/managedrun"
)

func TestRetainedOperationTimestampsSurviveLookupAndStreamReplay(t *testing.T) {
	sr := newServiceRunner(config.Configuration{}, func(config.Configuration) (managed.RunResult, error) { return managed.RunResult{}, nil })
	sr.registerTrackedOperation("op-time", "Slack", actionInstallItem)
	sr.appendOperationEvent("op-time", operationStatusEventPayload{State: "Completed", ItemName: "Slack", Action: "Install"})
	events, done, ok := sr.snapshotTrackedOperation("op-time")
	if !ok || !done || len(events) != 2 {
		t.Fatalf("unexpected retained events: %+v", events)
	}
	for replay := 0; replay < 2; replay++ {
		file, err := os.CreateTemp(t.TempDir(), "replay")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = file.Close() })
		if err := sr.writeStreamOperationStatusSequence(file, serviceEnvelope[json.RawMessage]{}, "op-time"); err != nil {
			t.Fatal(err)
		}
		if _, err := file.Seek(0, 0); err != nil {
			t.Fatal(err)
		}
		decoder := json.NewDecoder(file)
		var envelope serviceEnvelope[json.RawMessage]
		if err := decoder.Decode(&envelope); err != nil {
			t.Fatal(err)
		} // acknowledgement
		for _, event := range events {
			if err := decoder.Decode(&envelope); err != nil {
				t.Fatal(err)
			}
			if got, want := envelope.TimestampUTC, event.timestamp.UTC().Format(time.RFC3339Nano); got != want {
				t.Fatalf("replay %d timestamp = %q, want original %q", replay, got, want)
			}
		}
		if got, want := sr.snapshotTrackedOperations()[0].TimestampUTC, events[1].timestamp.UTC().Format(time.RFC3339Nano); got != want {
			t.Fatalf("lookup timestamp = %q, want original %q", got, want)
		}
	}
}

func TestSnapshotTrackedOperationsReturnsLatestRetainedState(t *testing.T) {
	sr := newServiceRunner(config.Configuration{}, func(config.Configuration) (managed.RunResult, error) { return managed.RunResult{}, nil })
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
	first := newServiceRunner(config.Configuration{}, func(config.Configuration) (managed.RunResult, error) { return managed.RunResult{}, nil })
	first.registerTrackedOperation("op-1", "Slack", actionInstallItem)
	if len(first.snapshotTrackedOperations()) != 1 {
		t.Fatal("expected first process to retain its operation")
	}

	restarted := newServiceRunner(config.Configuration{}, func(config.Configuration) (managed.RunResult, error) { return managed.RunResult{}, nil })
	if got := restarted.snapshotTrackedOperations(); len(got) != 0 {
		t.Fatalf("expected restarted service to begin with empty operation registry, got %+v", got)
	}
}
