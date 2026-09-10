//go:build windows

package service

import (
	"encoding/json"
	"os"
	"sort"
	"time"
)

const actionListOperations = "ListOperations"

type listOperationsRequest struct{}

type operationSnapshotPayload struct {
	OperationID string                      `json:"operationId"`
	Status      operationStatusEventPayload `json:"status"`
}

type listOperationsResponse struct {
	Operations []operationSnapshotPayload `json:"operations"`
}

// snapshotTrackedOperations returns the latest known state for each retained
// operation. Operation history is deliberately process-local: a service restart
// starts with an empty registry and normal managed convergence resumes from
// persistent policy plus fresh detection instead of replaying old mutations.
func (sr *serviceRunner) snapshotTrackedOperations() []operationSnapshotPayload {
	sr.operationsMu.Lock()
	defer sr.operationsMu.Unlock()
	sr.pruneTrackedOperationsLocked(time.Now())

	out := make([]operationSnapshotPayload, 0, len(sr.operations))
	for operationID, op := range sr.operations {
		if len(op.events) == 0 {
			continue
		}
		out = append(out, operationSnapshotPayload{
			OperationID: operationID,
			Status:      op.events[len(op.events)-1],
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].OperationID < out[j].OperationID
	})
	return out
}

func (sr *serviceRunner) writeListOperationsResponse(file *os.File, req serviceEnvelope[json.RawMessage]) error {
	return json.NewEncoder(file).Encode(serviceEnvelope[listOperationsResponse]{
		Version:      pipeProtocolVersion,
		MessageType:  messageTypeResponse,
		Operation:    actionListOperations,
		RequestID:    req.RequestID,
		OperationID:  "",
		TimestampUTC: nowRFC3339UTC(),
		Payload: listOperationsResponse{
			Operations: sr.snapshotTrackedOperations(),
		},
	})
}
