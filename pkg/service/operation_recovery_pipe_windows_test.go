//go:build windows

package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/1dustindavis/gorilla/pkg/config"
)

func TestNamedPipeDuplicateMutationReturnsOriginalOperation(t *testing.T) {
	stubOptionalSlack(t)
	cfg := config.Configuration{
		AppDataPath:     t.TempDir(),
		ServicePipeName: fmt.Sprintf("gorilla-stage4-recovery-%d", time.Now().UnixNano()),
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

	const mutationID = "mutation-stage4-recovery"
	request := func(requestID string) serviceEnvelope[installItemRequest] {
		return serviceEnvelope[installItemRequest]{
			Version:      pipeProtocolVersion,
			MessageType:  messageTypeRequest,
			Operation:    actionInstallItem,
			RequestID:    requestID,
			TimestampUTC: nowRFC3339UTC(),
			Payload: installItemRequest{
				ItemName:   "Slack",
				MutationID: mutationID,
			},
		}
	}

	first := sendOneRequest(t, cfg, request("req-stage4-first"))
	if first.MessageType != messageTypeResponse {
		t.Fatalf("first request: expected %s, got %s", messageTypeResponse, first.MessageType)
	}
	if strings.TrimSpace(first.OperationID) == "" {
		t.Fatal("first request returned empty operationId")
	}

	second := sendOneRequest(t, cfg, request("req-stage4-retry"))
	if second.MessageType != messageTypeResponse {
		t.Fatalf("retry request: expected %s, got %s", messageTypeResponse, second.MessageType)
	}
	if second.OperationID != first.OperationID {
		t.Fatalf("retry with same mutationId created a different operation: first=%s retry=%s", first.OperationID, second.OperationID)
	}

	sr.operationsMu.Lock()
	trackedCount := len(sr.operations)
	sr.operationsMu.Unlock()
	if trackedCount != 1 {
		t.Fatalf("expected exactly one tracked operation after duplicate mutation, got %d", trackedCount)
	}
}
