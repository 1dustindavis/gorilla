package service

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/1dustindavis/gorilla/pkg/appcatalog"
)

const (
	pipeProtocolVersion = "v1"

	messageTypeRequest  = "Request"
	messageTypeResponse = "Response"
	messageTypeEvent    = "Event"
	messageTypeError    = "Error"
)

type serviceEnvelope[T any] struct {
	Version      string `json:"version"`
	MessageType  string `json:"messageType"`
	Operation    string `json:"operation"`
	RequestID    string `json:"requestId"`
	OperationID  string `json:"operationId"`
	TimestampUTC string `json:"timestampUtc"`
	Payload      T      `json:"payload"`
}

type listOptionalInstallsRequest struct{}

type installItemRequest struct {
	ItemName   string `json:"itemName"`
	MutationID string `json:"mutationId"`
}

func (r *installItemRequest) UnmarshalJSON(data []byte) error {
	type wire installItemRequest
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if strings.TrimSpace(decoded.MutationID) == "" {
		// v1 callers predating mutation identity remain usable, but only callers
		// that supply a stable mutationId can safely reconcile an uncertain ack.
		decoded.MutationID = newRequestID()
	}
	*r = installItemRequest(decoded)
	return nil
}

type removeItemRequest struct {
	ItemName   string `json:"itemName"`
	MutationID string `json:"mutationId"`
}

func (r *removeItemRequest) UnmarshalJSON(data []byte) error {
	type wire removeItemRequest
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if strings.TrimSpace(decoded.MutationID) == "" {
		decoded.MutationID = newRequestID()
	}
	*r = removeItemRequest(decoded)
	return nil
}

type streamOperationStatusRequest struct{}

type optionalInstallResponseItem struct {
	ItemName           string                 `json:"itemName"`
	DisplayName        string                 `json:"displayName"`
	Version            string                 `json:"version"`
	Catalog            string                 `json:"catalog"`
	InstallerType      string                 `json:"installerType"`
	InstallerPackageID string                 `json:"installerPackageId"`
	InstallerLocation  string                 `json:"installerLocation"`
	IsManaged          bool                   `json:"isManaged"`
	IsInstalled        bool                   `json:"isInstalled"`
	Status             string                 `json:"status"`
	StatusUpdatedAtUTC string                 `json:"statusUpdatedAtUtc"`
	LastOperationID    string                 `json:"lastOperationId,omitempty"`
	TargetVersion      *string                `json:"targetVersion"`
	Observation        appcatalog.Observation `json:"observation"`
	Policy             appcatalog.Policy      `json:"policy"`
	Actions            appcatalog.Actions     `json:"actions"`
}

type listOptionalInstallsResponse struct {
	Items []optionalInstallResponseItem `json:"items"`
}

type operationAcceptedResponse struct {
	Accepted    bool   `json:"accepted"`
	QueuedAtUTC string `json:"queuedAtUtc"`
}

type streamOperationStatusAckResponse struct {
	StreamAccepted bool `json:"streamAccepted"`
}

// operationStatusEventPayload uses state only for lifecycle. Every event carries
// item/action identity, terminal Completed events carry the authoritative result,
// and progress is omitted unless it is genuinely measured.
type operationStatusEventPayload struct {
	State           string                  `json:"state"`
	ProgressPercent int                     `json:"progressPercent,omitempty"`
	Message         string                  `json:"message"`
	ItemName        string                  `json:"itemName"`
	Action          appcatalog.Action       `json:"action"`
	Result          *operationResultPayload `json:"result,omitempty"`
}

type errorResponsePayload struct {
	ErrorCode    string `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
}

func nowRFC3339UTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func newRequestID() string {
	return strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
}

func decodeEnvelopePayload[T any](raw json.RawMessage) (T, error) {
	var payload T
	if len(raw) == 0 || string(raw) == "null" {
		return payload, nil
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return payload, err
	}
	return payload, nil
}
