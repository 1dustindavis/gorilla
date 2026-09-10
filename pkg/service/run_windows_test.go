//go:build windows

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/1dustindavis/gorilla/pkg/appcatalog"
	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/manifest"
	"github.com/1dustindavis/gorilla/pkg/status"
	"golang.org/x/sys/windows"
)

func TestFlushAndDisconnectNamedPipeStillDisconnectsWhenFlushReportsBrokenPipe(t *testing.T) {
	var calls []string

	originalFlush := flushNamedPipeBuffers
	originalDisconnect := disconnectNamedPipe
	t.Cleanup(func() {
		flushNamedPipeBuffers = originalFlush
		disconnectNamedPipe = originalDisconnect
	})

	flushNamedPipeBuffers = func(_ windows.Handle) error {
		calls = append(calls, "flush")
		return windows.ERROR_BROKEN_PIPE
	}
	disconnectNamedPipe = func(_ windows.Handle) error {
		calls = append(calls, "disconnect")
		return windows.ERROR_PIPE_NOT_CONNECTED
	}

	sr := &serviceRunner{}
	sr.flushAndDisconnectNamedPipe(windows.InvalidHandle)

	if len(calls) != 2 {
		t.Fatalf("expected exactly two pipe calls, got %d (%v)", len(calls), calls)
	}
	if calls[0] != "flush" || calls[1] != "disconnect" {
		t.Fatalf("expected call order flush -> disconnect, got %v", calls)
	}
}

func TestServiceStartExplainsLegacyUninstallMigrationFailure(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir()}
	path := serviceLocalManifestPath(cfg)
	if err := os.WriteFile(path, []byte("name: ["), 0600); err != nil {
		t.Fatal(err)
	}

	err := newServiceRunner(cfg, func(config.Configuration) error { return nil }).start(context.Background())
	if err == nil {
		t.Fatal("expected invalid legacy service manifest to stop startup")
	}
	message := err.Error()
	for _, expected := range []string{
		"persistent uninstall requests created by an older App Catalog version",
		path,
		"service will not start",
		"repeatedly uninstall software",
		"unable to parse service local manifest",
	} {
		if !strings.Contains(message, expected) {
			t.Fatalf("startup error %q does not explain %q", message, expected)
		}
	}
}

func TestNamedPipeStreamStatusReliability(t *testing.T) {
	stubOptionalSlack(t)
	tempDir := t.TempDir()
	cfg := config.Configuration{
		AppDataPath:     tempDir,
		ServicePipeName: fmt.Sprintf("gorilla-test-%d", time.Now().UnixNano()),
		ServiceInterval: "1h",
		ServiceMode:     true,
		ServiceName:     "gorilla-test",
	}

	sr := newServiceRunner(cfg, func(config.Configuration) error { return nil })
	ctx, cancel := context.WithCancel(context.Background())

	if err := sr.start(ctx); err != nil {
		t.Fatalf("service start failed: %v", err)
	}
	defer func() {
		cancel()
		bestEffortUnblockPipeListener(cfg)
		sr.stop(context.Background())
	}()

	iterations := namedPipeReliabilityIterations(t)
	for i := 0; i < iterations; i++ {
		operationID := mustInstallAndGetOperationID(t, cfg, i)
		mustStreamAndReceiveTerminalEvent(t, cfg, operationID, i)
	}
}

func TestStreamOperationStatusUnknownOperationIDReturnsError(t *testing.T) {
	tempDir := t.TempDir()
	cfg := config.Configuration{
		AppDataPath:     tempDir,
		ServicePipeName: fmt.Sprintf("gorilla-test-%d", time.Now().UnixNano()),
		ServiceInterval: "1h",
		ServiceMode:     true,
		ServiceName:     "gorilla-test",
	}

	sr := newServiceRunner(cfg, func(config.Configuration) error { return nil })
	ctx, cancel := context.WithCancel(context.Background())

	if err := sr.start(ctx); err != nil {
		t.Fatalf("service start failed: %v", err)
	}
	defer func() {
		cancel()
		bestEffortUnblockPipeListener(cfg)
		sr.stop(context.Background())
	}()

	conn, err := openPipe(servicePipePath(cfg.ServicePipeName), 5*time.Second)
	if err != nil {
		t.Fatalf("failed to open service pipe: %v", err)
	}
	defer func() { _ = conn.Close() }()

	request := serviceEnvelope[streamOperationStatusRequest]{
		Version:      pipeProtocolVersion,
		MessageType:  messageTypeRequest,
		Operation:    actionStreamOperationStatus,
		RequestID:    "req-stream-unknown",
		OperationID:  "does-not-exist",
		TimestampUTC: nowRFC3339UTC(),
		Payload:      streamOperationStatusRequest{},
	}
	if err := json.NewEncoder(conn).Encode(request); err != nil {
		t.Fatalf("failed to encode stream request: %v", err)
	}

	var resp serviceEnvelope[errorResponsePayload]
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		t.Fatalf("failed to decode stream error response: %v", err)
	}
	if resp.MessageType != messageTypeError {
		t.Fatalf("expected messageType=%s, got %s", messageTypeError, resp.MessageType)
	}
	if resp.Payload.ErrorCode != "invalid_request" {
		t.Fatalf("expected errorCode=invalid_request, got %s", resp.Payload.ErrorCode)
	}
}

func TestStreamOperationStatusFailedLifecycle(t *testing.T) {
	stubOptionalSlack(t)
	tempDir := t.TempDir()
	cfg := config.Configuration{
		AppDataPath:     tempDir,
		ServicePipeName: fmt.Sprintf("gorilla-test-%d", time.Now().UnixNano()),
		ServiceInterval: "1h",
		ServiceMode:     true,
		ServiceName:     "gorilla-test",
	}

	sr := newServiceRunner(cfg, func(config.Configuration) error { return errors.New("forced managed run failure") })
	ctx, cancel := context.WithCancel(context.Background())

	if err := sr.start(ctx); err != nil {
		t.Fatalf("service start failed: %v", err)
	}
	defer func() {
		cancel()
		bestEffortUnblockPipeListener(cfg)
		sr.stop(context.Background())
	}()

	operationID := mustInstallAndGetOperationID(t, cfg, 0)
	terminal := mustStreamAndReceiveTerminalState(t, cfg, operationID, 0)
	if terminal.State != "Completed" {
		t.Fatalf("expected terminal state Completed, got %s", terminal.State)
	}
	if terminal.Result == nil || terminal.Result.Outcome != appcatalog.Failed || terminal.Result.DetailCode != "managed_run_failed" {
		t.Fatalf("expected structured managed-run failure, got %+v", terminal.Result)
	}
}

func stubOptionalSlack(t *testing.T) {
	t.Helper()
	stubOptionalCatalog(t,
		[]manifest.Item{{OptionalInstalls: []string{"Slack"}}},
		map[int]map[string]catalog.Item{1: {"Slack": {
			DisplayName: "Slack", Installer: catalog.InstallerItem{Type: "msi", Location: "slack.msi"},
		}}},
		map[string]status.Observation{"Slack": {State: status.Absent, CheckedAtUTC: time.Now().UTC()}},
	)
}

func TestScheduleRunAfterMutationEmitsInterruptedResult(t *testing.T) {
	sr := newServiceRunner(config.Configuration{}, func(config.Configuration) error { return nil })
	operationID := "op-canceled"
	sr.registerTrackedOperation(operationID)

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	sr.scheduleRunAfterMutation(canceledCtx, actionInstallItem, CommandResponse{OperationID: operationID})
	sr.wg.Wait()

	events, done, ok := sr.snapshotTrackedOperation(operationID)
	if !ok {
		t.Fatalf("expected tracked operation to exist")
	}
	if !done {
		t.Fatalf("expected tracked operation to be marked done")
	}
	last := events[len(events)-1]
	if last.State != "Completed" {
		t.Fatalf("expected terminal state Completed, got %s", last.State)
	}
	if last.Result == nil || last.Result.Outcome != appcatalog.Interrupted || last.Result.DetailCode != "service_canceled" {
		t.Fatalf("expected structured interrupted result, got %+v", last.Result)
	}
}

func TestListEnvelopeCarriesRealContractData(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "list-response-*.json")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC)
	target, installed := "2.0", "1.7"
	contract := appcatalog.Item{
		ItemName: "Example", DisplayName: "Example App", Catalog: "production", TargetVersion: &target,
		Observation: appcatalog.Observation{
			State: appcatalog.UpdateAvailable, InstalledVersion: &installed, CheckedAtUTC: &now,
			InstallRequirement: appcatalog.RequirementNotSatisfied,
		},
		Policy: appcatalog.Policy{Optional: true, Selection: appcatalog.NoSelection},
	}
	contract.Actions = appcatalog.DecideActions(contract.Observation.State, contract.Policy, appcatalog.Capabilities{CanInstall: true, CanRemove: true}, false)
	sr := &serviceRunner{}
	req := serviceEnvelope[json.RawMessage]{RequestID: "req-list", Operation: actionListOptionalInstalls}
	resp := CommandResponse{OptionalItems: []optionalItemDetails{{Contract: contract, InstallerType: "msi", InstallerLocation: "example.msi"}}}
	if err := sr.writeSuccessEnvelope(file, req, Command{Action: actionListOptionalInstalls}, resp); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	var envelope serviceEnvelope[listOptionalInstallsResponse]
	if err := json.NewDecoder(file).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	got := envelope.Payload.Items[0]
	if got.DisplayName != "Example App" || got.Version != "2.0" || got.Catalog != "production" || !got.IsInstalled || got.Status != "UpdateAvailable" {
		t.Fatalf("legacy fields lost real data: %+v", got)
	}
	if got.Observation.State != appcatalog.UpdateAvailable || got.Policy.Optional != true || !got.Actions.Install.Allowed || !got.Actions.Remove.Allowed {
		t.Fatalf("contract fields missing: %+v", got)
	}
}

func namedPipeReliabilityIterations(t *testing.T) int {
	t.Helper()

	const (
		defaultIterations = 10
		shortIterations   = 2
		envKey            = "GORILLA_SERVICE_PIPE_RELIABILITY_ITERATIONS"
	)

	iterations := defaultIterations
	if value := strings.TrimSpace(os.Getenv(envKey)); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			t.Fatalf("invalid %s value %q: expected positive integer", envKey, value)
		}
		iterations = parsed
	}

	if testing.Short() && iterations > shortIterations {
		return shortIterations
	}

	return iterations
}

func mustInstallAndGetOperationID(t *testing.T, cfg config.Configuration, seq int) string {
	t.Helper()

	request := serviceEnvelope[installItemRequest]{
		Version:      pipeProtocolVersion,
		MessageType:  messageTypeRequest,
		Operation:    actionInstallItem,
		RequestID:    fmt.Sprintf("req-install-%d", seq),
		OperationID:  "",
		TimestampUTC: nowRFC3339UTC(),
		Payload: installItemRequest{
			ItemName: "Slack",
		},
	}

	response := sendOneRequest(t, cfg, request)
	if response.MessageType != messageTypeResponse {
		t.Fatalf("expected %s message type, got %s", messageTypeResponse, response.MessageType)
	}
	if response.Operation != actionInstallItem {
		t.Fatalf("expected operation %s, got %s", actionInstallItem, response.Operation)
	}
	if strings.TrimSpace(response.OperationID) == "" {
		t.Fatalf("expected non-empty operationId from install response")
	}

	return response.OperationID
}

func mustStreamAndReceiveTerminalEvent(t *testing.T, cfg config.Configuration, operationID string, seq int) {
	t.Helper()

	terminal := mustStreamAndReceiveTerminalState(t, cfg, operationID, seq)
	if terminal.State != "Completed" || terminal.Result == nil || (terminal.Result.Outcome != appcatalog.Succeeded && terminal.Result.Outcome != appcatalog.AlreadySatisfied) {
		t.Fatalf("expected completed successful result, got %+v", terminal)
	}
}

func mustStreamAndReceiveTerminalState(t *testing.T, cfg config.Configuration, operationID string, seq int) operationStatusEventPayload {
	t.Helper()

	conn, err := openPipe(servicePipePath(cfg.ServicePipeName), 5*time.Second)
	if err != nil {
		t.Fatalf("failed to open service pipe: %v", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	request := serviceEnvelope[streamOperationStatusRequest]{
		Version:      pipeProtocolVersion,
		MessageType:  messageTypeRequest,
		Operation:    actionStreamOperationStatus,
		RequestID:    fmt.Sprintf("req-stream-%d", seq),
		OperationID:  operationID,
		TimestampUTC: nowRFC3339UTC(),
		Payload:      streamOperationStatusRequest{},
	}

	if err := json.NewEncoder(conn).Encode(request); err != nil {
		t.Fatalf("failed to encode stream request: %v", err)
	}
	decoder := json.NewDecoder(conn)

	var ack serviceEnvelope[json.RawMessage]
	if err := decoder.Decode(&ack); err != nil {
		t.Fatalf("failed to decode stream ack: %v", err)
	}
	if ack.MessageType != messageTypeResponse {
		t.Fatalf("expected stream ack messageType=%s, got %s", messageTypeResponse, ack.MessageType)
	}
	if ack.Operation != actionStreamOperationStatus {
		t.Fatalf("expected stream ack operation=%s, got %s", actionStreamOperationStatus, ack.Operation)
	}
	if ack.OperationID != operationID {
		t.Fatalf("expected stream ack operationId=%s, got %s", operationID, ack.OperationID)
	}

	states := make([]string, 0, 4)
	var terminal operationStatusEventPayload
	for {
		var event serviceEnvelope[operationStatusEventPayload]
		if err := decoder.Decode(&event); err != nil {
			t.Fatalf("failed to decode stream event: %v", err)
		}
		if event.MessageType != messageTypeEvent {
			t.Fatalf("expected stream event messageType=%s, got %s", messageTypeEvent, event.MessageType)
		}
		if event.Operation != actionStreamOperationStatus {
			t.Fatalf("expected stream event operation=%s, got %s", actionStreamOperationStatus, event.Operation)
		}
		if event.OperationID != operationID {
			t.Fatalf("expected stream event operationId=%s, got %s", operationID, event.OperationID)
		}
		states = append(states, event.Payload.State)
		if event.Payload.State == "Completed" {
			terminal = event.Payload
			break
		}
	}

	if len(states) < 3 {
		t.Fatalf("expected multiple lifecycle states, got %v", states)
	}
	if states[0] != "Queued" {
		t.Fatalf("expected first state Queued, got %s (%v)", states[0], states)
	}
	return terminal
}

func sendOneRequest[T any](t *testing.T, cfg config.Configuration, req serviceEnvelope[T]) serviceEnvelope[json.RawMessage] {
	t.Helper()

	conn, err := openPipe(servicePipePath(cfg.ServicePipeName), 5*time.Second)
	if err != nil {
		t.Fatalf("failed to open service pipe: %v", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		t.Fatalf("failed to encode request: %v", err)
	}

	var resp serviceEnvelope[json.RawMessage]
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return resp
}

func bestEffortUnblockPipeListener(cfg config.Configuration) {
	conn, err := openPipe(servicePipePath(cfg.ServicePipeName), 250*time.Millisecond)
	if err != nil {
		return
	}
	_ = conn.Close()
}

func TestTrackedOperationPruningDropsOldCompletedEntries(t *testing.T) {
	sr := newServiceRunner(config.Configuration{}, func(config.Configuration) error { return nil })
	now := time.Now()

	sr.operationsMu.Lock()
	for i := 0; i < trackedOperationsMaxCount+50; i++ {
		id := fmt.Sprintf("done-%d", i)
		sr.operations[id] = &trackedOperation{
			events:      []operationStatusEventPayload{{State: "Completed", Message: "done"}},
			done:        true,
			lastUpdated: now.Add(-time.Duration(i) * time.Minute),
			completedAt: now.Add(-time.Duration(i) * time.Minute),
		}
	}
	sr.operations["active-op"] = &trackedOperation{
		events:      []operationStatusEventPayload{{State: "Installing", Message: "running"}},
		done:        false,
		lastUpdated: now,
	}
	sr.pruneTrackedOperationsLocked(now)
	_, activeStillTracked := sr.operations["active-op"]
	count := len(sr.operations)
	sr.operationsMu.Unlock()

	if !activeStillTracked {
		t.Fatalf("expected active operation to remain tracked after pruning")
	}
	if count > trackedOperationsMaxCount {
		t.Fatalf("expected tracked operations count <= %d, got %d", trackedOperationsMaxCount, count)
	}
}
