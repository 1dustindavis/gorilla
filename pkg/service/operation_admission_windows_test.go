//go:build windows

package service

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/config"
)

func TestMutationAdmissionReusesSameMutation(t *testing.T) {
	stubOptionalSlack(t)
	cfg := config.Configuration{AppDataPath: t.TempDir()}
	sr := newServiceRunner(cfg, func(config.Configuration) error { return nil })
	cmd := Command{
		Action:     actionInstallItem,
		Items:      []string{"Slack"},
		MutationID: "mutation-1",
	}

	first, err := sr.executeCommandSafe(cmd)
	if err != nil {
		t.Fatalf("first admission failed: %v", err)
	}
	second, err := sr.executeCommandSafe(cmd)
	if err != nil {
		t.Fatalf("retry admission failed: %v", err)
	}
	if first.OperationID == "" {
		t.Fatal("expected first admission to allocate operation ID")
	}
	if second.OperationID != first.OperationID {
		t.Fatalf("expected retry to reuse operation %q, got %q", first.OperationID, second.OperationID)
	}
	if !second.ReusedOperation {
		t.Fatal("expected retry to be marked as reused operation")
	}

	events, _, ok := sr.snapshotTrackedOperation(first.OperationID)
	if !ok {
		t.Fatal("expected admitted operation to be tracked")
	}
	if len(events) != 1 || events[0].State != "Queued" {
		t.Fatalf("expected one queued event after idempotent retry, got %+v", events)
	}

	selected, err := listServiceManagedInstalls(cfg)
	if err != nil {
		t.Fatalf("read managed selection: %v", err)
	}
	if len(selected) != 1 || selected[0] != "Slack" {
		t.Fatalf("expected Slack selected exactly once, got %v", selected)
	}
}

func TestMutationAdmissionRejectsConcurrentMutationForSameItem(t *testing.T) {
	stubOptionalSlack(t)
	sr := newServiceRunner(config.Configuration{AppDataPath: t.TempDir()}, func(config.Configuration) error { return nil })

	_, err := sr.executeCommandSafe(Command{
		Action: actionInstallItem, Items: []string{"Slack"}, MutationID: "mutation-1",
	})
	if err != nil {
		t.Fatalf("first admission failed: %v", err)
	}
	_, err = sr.executeCommandSafe(Command{
		Action: actionInstallItem, Items: []string{"Slack"}, MutationID: "mutation-2",
	})
	var denied actionDeniedError
	if !errors.As(err, &denied) || denied.reason != "operation_active" {
		t.Fatalf("expected operation_active denial, got %v", err)
	}
}

func TestMutationAdmissionRejectsMutationIDReuseForDifferentIdentity(t *testing.T) {
	stubOptionalSlack(t)
	sr := newServiceRunner(config.Configuration{AppDataPath: t.TempDir()}, func(config.Configuration) error { return nil })

	_, err := sr.executeCommandSafe(Command{
		Action: actionInstallItem, Items: []string{"Slack"}, MutationID: "mutation-1",
	})
	if err != nil {
		t.Fatalf("first admission failed: %v", err)
	}
	_, err = sr.executeCommandSafe(Command{
		Action: actionRemoveItem, Items: []string{"Slack"}, MutationID: "mutation-1",
	})
	var denied actionDeniedError
	if !errors.As(err, &denied) || denied.reason != "mutation_id_conflict" {
		t.Fatalf("expected mutation_id_conflict denial, got %v", err)
	}
}

func TestCommandFromRequestEnvelopeRejectsMissingMutationID(t *testing.T) {
	payload, err := json.Marshal(struct {
		ItemName string `json:"itemName"`
	}{ItemName: "Slack"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = commandFromRequestEnvelope(serviceEnvelope[json.RawMessage]{
		Operation: actionInstallItem,
		Payload:   payload,
	})
	if err == nil {
		t.Fatal("expected request without mutationId to be rejected")
	}
}

func TestCommandFromRequestEnvelopeCarriesMutationID(t *testing.T) {
	payload, err := json.Marshal(installItemRequest{ItemName: "Slack", MutationID: "mutation-1"})
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := commandFromRequestEnvelope(serviceEnvelope[json.RawMessage]{
		Operation: actionInstallItem,
		Payload:   payload,
	})
	if err != nil {
		t.Fatalf("map request: %v", err)
	}
	if cmd.MutationID != "mutation-1" {
		t.Fatalf("expected mutation ID to survive request mapping, got %q", cmd.MutationID)
	}
}
