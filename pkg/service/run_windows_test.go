//go:build windows

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/1dustindavis/gorilla/pkg/config"
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

func TestNamedPipeStreamStatusReliability(t *testing.T) {
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
	defer cancel()

	if err := sr.start(ctx); err != nil {
		t.Fatalf("service start failed: %v", err)
	}
	defer sr.stop(context.Background())

	iterations := 25
	for i := 0; i < iterations; i++ {
		operationID := mustInstallAndGetOperationID(t, cfg, i)
		mustStreamAndReceiveTerminalEvent(t, cfg, operationID)
	}
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

func mustStreamAndReceiveTerminalEvent(t *testing.T, cfg config.Configuration, operationID string) {
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
		RequestID:    fmt.Sprintf("req-stream-%d", time.Now().UnixNano()),
		OperationID:  operationID,
		TimestampUTC: nowRFC3339UTC(),
		Payload:      streamOperationStatusRequest{},
	}

	if err := json.NewEncoder(conn).Encode(request); err != nil {
		t.Fatalf("failed to encode stream request: %v", err)
	}

	var ack serviceEnvelope[json.RawMessage]
	if err := json.NewDecoder(conn).Decode(&ack); err != nil {
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

	var event serviceEnvelope[operationStatusEventPayload]
	if err := json.NewDecoder(conn).Decode(&event); err != nil {
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
	if event.Payload.State != "Succeeded" {
		t.Fatalf("expected stream terminal state Succeeded, got %s", event.Payload.State)
	}
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
