//go:build windows

package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/1dustindavis/gorilla/pkg/config"
	"golang.org/x/sys/windows"
)

func sendCommand(cfg config.Configuration, cmd Command) (CommandResponse, error) {
	pipePath := servicePipePath(cfg.ServicePipeName)
	conn, err := openPipe(pipePath, 30*time.Second)
	if err != nil {
		return CommandResponse{}, fmt.Errorf("failed to connect to service pipe %s: %w", pipePath, err)
	}
	defer func() {
		_ = conn.Close()
	}()

	requestEnvelope, err := makeRequestEnvelope(cmd)
	if err != nil {
		return CommandResponse{}, err
	}

	if err := json.NewEncoder(conn).Encode(requestEnvelope); err != nil {
		return CommandResponse{}, fmt.Errorf("failed to send service command: %w", err)
	}

	var rawResp serviceEnvelope[json.RawMessage]
	if err := json.NewDecoder(conn).Decode(&rawResp); err != nil {
		return CommandResponse{}, fmt.Errorf("failed to decode service response: %w", err)
	}

	switch rawResp.MessageType {
	case messageTypeError:
		errPayload, decodeErr := decodeEnvelopePayload[errorResponsePayload](rawResp.Payload)
		if decodeErr != nil {
			return CommandResponse{}, fmt.Errorf("failed to decode service error payload: %w", decodeErr)
		}
		if strings.TrimSpace(errPayload.ErrorMessage) == "" {
			errPayload.ErrorMessage = "service command failed"
		}
		return CommandResponse{}, errors.New(errPayload.ErrorMessage)
	case messageTypeResponse:
		resp, mapErr := mapEnvelopeToCommandResponse(rawResp)
		if mapErr != nil {
			return CommandResponse{}, mapErr
		}
		return resp, nil
	default:
		return CommandResponse{}, fmt.Errorf("unsupported service messageType %q", rawResp.MessageType)
	}
}

func servicePipePath(pipeName string) string {
	if strings.HasPrefix(pipeName, `\\.\pipe\`) {
		return pipeName
	}
	return `\\.\pipe\` + strings.TrimSpace(pipeName)
}

func openPipe(pipePath string, timeout time.Duration) (*os.File, error) {
	deadline := time.Now().Add(timeout)
	for {
		pathPtr, err := windows.UTF16PtrFromString(pipePath)
		if err != nil {
			return nil, err
		}

		handle, err := windows.CreateFile(
			pathPtr,
			windows.GENERIC_READ|windows.GENERIC_WRITE,
			0,
			nil,
			windows.OPEN_EXISTING,
			windows.FILE_ATTRIBUTE_NORMAL,
			0,
		)
		if err == nil {
			return os.NewFile(uintptr(handle), pipePath), nil
		}

		if !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) && !errors.Is(err, windows.ERROR_PIPE_BUSY) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, err
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func makeRequestEnvelope(cmd Command) (serviceEnvelope[any], error) {
	envelope := serviceEnvelope[any]{
		Version:      pipeProtocolVersion,
		MessageType:  messageTypeRequest,
		Operation:    cmd.Action,
		RequestID:    newRequestID(),
		OperationID:  "",
		TimestampUTC: nowRFC3339UTC(),
		Payload:      listOptionalInstallsRequest{},
	}

	switch cmd.Action {
	case actionListOptionalInstalls:
		envelope.Payload = listOptionalInstallsRequest{}
	case actionInstallItem:
		envelope.Payload = installItemRequest{ItemName: cmd.Items[0]}
	case actionRemoveItem:
		envelope.Payload = removeItemRequest{ItemName: cmd.Items[0]}
	case actionStreamOperationStatus:
		envelope.OperationID = cmd.Items[0]
		envelope.Payload = streamOperationStatusRequest{}
	default:
		return serviceEnvelope[any]{}, fmt.Errorf("unsupported service action %q", cmd.Action)
	}

	return envelope, nil
}

func mapEnvelopeToCommandResponse(raw serviceEnvelope[json.RawMessage]) (CommandResponse, error) {
	resp := CommandResponse{Status: "ok", OperationID: raw.OperationID}

	switch raw.Operation {
	case actionListOptionalInstalls:
		payload, err := decodeEnvelopePayload[listOptionalInstallsResponse](raw.Payload)
		if err != nil {
			return CommandResponse{}, fmt.Errorf("failed to decode ListOptionalInstalls payload: %w", err)
		}
		items := make([]string, 0, len(payload.Items))
		for _, item := range payload.Items {
			if strings.TrimSpace(item.ItemName) != "" {
				items = append(items, item.ItemName)
			}
		}
		slices.Sort(items)
		resp.Items = items
		return resp, nil
	case actionInstallItem, actionRemoveItem:
		payload, err := decodeEnvelopePayload[operationAcceptedResponse](raw.Payload)
		if err != nil {
			return CommandResponse{}, fmt.Errorf("failed to decode operation accepted payload: %w", err)
		}
		if !payload.Accepted {
			return CommandResponse{}, errors.New("service did not accept operation")
		}
		return resp, nil
	case actionStreamOperationStatus:
		payload, err := decodeEnvelopePayload[streamOperationStatusAckResponse](raw.Payload)
		if err != nil {
			return CommandResponse{}, fmt.Errorf("failed to decode stream ack payload: %w", err)
		}
		if !payload.StreamAccepted {
			return CommandResponse{}, errors.New("service rejected stream request")
		}
		resp.Message = "StreamOperationStatus acknowledged by service"
		return resp, nil
	default:
		return CommandResponse{}, fmt.Errorf("unsupported response operation %q", raw.Operation)
	}
}
