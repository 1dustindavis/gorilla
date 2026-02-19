//go:build windows

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
)

type queuedCommand struct {
	cmd    Command
	result chan queuedResult
}

type queuedResult struct {
	resp CommandResponse
	err  error
}

type serviceRunner struct {
	cfg                config.Configuration
	managedRun         func(config.Configuration) error
	queue              chan queuedCommand
	handlerSem         chan struct{}
	wg                 sync.WaitGroup
	execMutex          sync.Mutex
	pipeListenerMu     sync.Mutex
	pipeListenerHandle windows.Handle
	activeConnMu       sync.Mutex
	activeConns        map[windows.Handle]struct{}
	operationsMu       sync.Mutex
	operations         map[string]*trackedOperation
}

var (
	flushNamedPipeBuffers = windows.FlushFileBuffers
	disconnectNamedPipe   = windows.DisconnectNamedPipe
	streamPollSleep       = 20 * time.Millisecond
)

const (
	maxConcurrentPipeHandlers    = 32
	trackedOperationsMaxCount    = 512
	trackedCompletedOperationTTL = 24 * time.Hour
)

type trackedOperation struct {
	events      []operationStatusEventPayload
	done        bool
	lastUpdated time.Time
	completedAt time.Time
}

func newServiceRunner(cfg config.Configuration, managedRun func(config.Configuration) error) *serviceRunner {
	return &serviceRunner{
		cfg:         cfg,
		managedRun:  managedRun,
		queue:       make(chan queuedCommand),
		handlerSem:  make(chan struct{}, maxConcurrentPipeHandlers),
		activeConns: make(map[windows.Handle]struct{}),
		operations:  make(map[string]*trackedOperation),
	}
}

func (sr *serviceRunner) start(ctx context.Context) error {
	if err := gorillalog.NewLog(sr.cfg); err != nil {
		return fmt.Errorf("initialize logger: %w", err)
	}

	interval, err := time.ParseDuration(sr.cfg.ServiceInterval)
	if err != nil || interval <= 0 {
		return fmt.Errorf("invalid service interval %q: %w", sr.cfg.ServiceInterval, err)
	}

	sr.wg.Add(1)
	go func() {
		defer sr.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case queued := <-sr.queue:
				sr.execMutex.Lock()
				resp, err := sr.executeCommandSafe(queued.cmd)
				sr.execMutex.Unlock()
				queued.result <- queuedResult{resp: resp, err: err}
			}
		}
	}()

	sr.wg.Add(1)
	go func() {
		defer sr.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		_, _ = sr.submit(ctx, Command{Action: "run"})
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _ = sr.submit(ctx, Command{Action: "run"})
			}
		}
	}()

	sr.wg.Add(1)
	go func() {
		defer sr.wg.Done()
		err := sr.serveNamedPipe(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			gorillalog.Warn("service named pipe endpoint failed:", err)
		}
	}()

	return nil
}

func (sr *serviceRunner) executeCommandSafe(cmd Command) (resp CommandResponse, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			gorillalog.Warn("panic during service command execution:", recovered)
			gorillalog.Warn(string(debug.Stack()))
			resp = CommandResponse{}
			err = fmt.Errorf("internal service panic while executing action %q", cmd.Action)
		}
	}()

	return executeCommand(sr.cfg, cmd, sr.managedRun)
}

func (sr *serviceRunner) stop(ctx context.Context) {
	sr.closeListenerPipe()
	sr.closeActiveConnections()
	sr.wg.Wait()
	gorillalog.Close()
	_ = ctx
}

func (sr *serviceRunner) submit(ctx context.Context, cmd Command) (CommandResponse, error) {
	result := make(chan queuedResult, 1)
	select {
	case <-ctx.Done():
		return CommandResponse{}, ctx.Err()
	case sr.queue <- queuedCommand{cmd: cmd, result: result}:
	}

	select {
	case <-ctx.Done():
		return CommandResponse{}, ctx.Err()
	case out := <-result:
		return out.resp, out.err
	}
}

func writeErrorEnvelope(file *os.File, requestID, operation, operationID, code, message string) {
	if err := json.NewEncoder(file).Encode(serviceEnvelope[errorResponsePayload]{
		Version:      pipeProtocolVersion,
		MessageType:  messageTypeError,
		Operation:    operation,
		RequestID:    requestID,
		OperationID:  operationID,
		TimestampUTC: nowRFC3339UTC(),
		Payload: errorResponsePayload{
			ErrorCode:    code,
			ErrorMessage: message,
		},
	}); err != nil {
		gorillalog.Warn("failed to write error envelope:", err)
	}
}

func (sr *serviceRunner) serveNamedPipe(ctx context.Context) error {
	pipePath := servicePipePath(sr.cfg.ServicePipeName)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		handle, err := createNamedPipe(pipePath)
		if err != nil {
			return fmt.Errorf("create pipe: %w", err)
		}

		sr.pipeListenerMu.Lock()
		sr.pipeListenerHandle = handle
		sr.pipeListenerMu.Unlock()

		err = windows.ConnectNamedPipe(handle, nil)
		if err != nil && !errors.Is(err, windows.ERROR_PIPE_CONNECTED) {
			windows.CloseHandle(handle)
			sr.clearListenerPipe(handle)
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if errors.Is(err, windows.ERROR_NO_DATA) || errors.Is(err, windows.ERROR_OPERATION_ABORTED) {
				continue
			}
			return fmt.Errorf("connect pipe: %w", err)
		}

		sr.clearListenerPipe(handle)
		select {
		case sr.handlerSem <- struct{}{}:
			sr.trackActiveConnection(handle)
			sr.wg.Add(1)
			go func(connectedHandle windows.Handle) {
				defer sr.wg.Done()
				defer func() {
					sr.untrackActiveConnection(connectedHandle)
					<-sr.handlerSem
				}()
				file := os.NewFile(uintptr(connectedHandle), pipePath)
				sr.handlePipeCommand(ctx, file)
				sr.flushAndDisconnectNamedPipe(connectedHandle)
				_ = file.Close()
			}(handle)
		default:
			file := os.NewFile(uintptr(handle), pipePath)
			writeErrorEnvelope(file, "", actionStreamOperationStatus, "", "server_busy", "service is busy; retry shortly")
			sr.flushAndDisconnectNamedPipe(handle)
			_ = file.Close()
		}
	}
}

func (sr *serviceRunner) flushAndDisconnectNamedPipe(handle windows.Handle) {
	if err := flushNamedPipeBuffers(handle); err != nil &&
		!errors.Is(err, windows.ERROR_BROKEN_PIPE) &&
		!errors.Is(err, windows.ERROR_NO_DATA) {
		gorillalog.Warn("failed to flush named pipe buffers:", err)
	}

	if err := disconnectNamedPipe(handle); err != nil &&
		!errors.Is(err, windows.ERROR_PIPE_NOT_CONNECTED) &&
		!errors.Is(err, windows.ERROR_BROKEN_PIPE) &&
		!errors.Is(err, windows.ERROR_NO_DATA) {
		gorillalog.Warn("failed to disconnect named pipe:", err)
	}
}

func (sr *serviceRunner) handlePipeCommand(ctx context.Context, file *os.File) {
	startedAt := time.Now()
	result := "error"
	var req serviceEnvelope[json.RawMessage]
	defer func() {
		if recovered := recover(); recovered != nil {
			result = "error"
			gorillalog.Warn("panic while handling named pipe request:", recovered)
			gorillalog.Warn(string(debug.Stack()))
			writeErrorEnvelope(file, req.RequestID, req.Operation, req.OperationID, "internal_error", "internal service error")
		}
		gorillalog.Debug(
			"named pipe lifecycle:",
			"operation=", req.Operation,
			"requestId=", req.RequestID,
			"operationId=", req.OperationID,
			"state=completed",
			"result=", result,
			"durationMs=", time.Since(startedAt).Milliseconds(),
		)
	}()

	if err := json.NewDecoder(file).Decode(&req); err != nil {
		result = "error"
		gorillalog.Warn("failed to decode named pipe request:", err)
		writeErrorEnvelope(file, "", "", "", "invalid_request", "invalid JSON request body")
		return
	}

	// Keep correlation keys explicit so request/operation flow can be joined with UI diagnostics.
	gorillalog.Debug("named pipe request:", req.Operation, "requestId=", req.RequestID, "operationId=", req.OperationID)

	if req.Version != pipeProtocolVersion {
		result = "error"
		writeErrorEnvelope(file, req.RequestID, req.Operation, req.OperationID, "unsupported_version", "unsupported protocol version")
		return
	}

	cmd, err := commandFromRequestEnvelope(req)
	if err != nil {
		result = "error"
		gorillalog.Warn("failed to map request envelope to command:", err)
		writeErrorEnvelope(file, req.RequestID, req.Operation, req.OperationID, "invalid_request", err.Error())
		return
	}

	if err := validateCommand(cmd); err != nil {
		result = "error"
		gorillalog.Warn("command validation failed:", err)
		writeErrorEnvelope(file, req.RequestID, req.Operation, req.OperationID, "invalid_request", err.Error())
		return
	}

	if cmd.Action == actionStreamOperationStatus {
		if err := sr.writeStreamOperationStatusSequence(file, req, cmd.Items[0]); err != nil {
			result = "error"
			gorillalog.Warn("failed to write stream response envelope:", err)
			return
		}
		result = "ok"
		return
	}

	resp, err := sr.submit(ctx, cmd)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			result = "canceled"
		} else {
			result = "error"
		}
		gorillalog.Warn("command execution failed:", err)
		writeErrorEnvelope(file, req.RequestID, req.Operation, req.OperationID, "command_failed", err.Error())
		return
	}
	if cmd.Action == actionInstallItem || cmd.Action == actionRemoveItem {
		sr.registerTrackedOperation(resp.OperationID)
	}

	if err := sr.writeSuccessEnvelope(file, req, cmd, resp); err != nil {
		result = "error"
		gorillalog.Warn("failed to write success envelope:", err)
	} else {
		result = "ok"
		gorillalog.Debug("named pipe response sent:", req.Operation, "requestId=", req.RequestID)
	}
	sr.scheduleRunAfterMutation(ctx, cmd.Action, resp.OperationID)
}

func (sr *serviceRunner) scheduleRunAfterMutation(ctx context.Context, action, operationID string) {
	if action != actionInstallItem && action != actionRemoveItem {
		return
	}

	sr.wg.Add(1)
	go func() {
		defer sr.wg.Done()
		sr.appendOperationEvent(operationID, operationStatusEventPayload{
			State:           "Validating",
			ProgressPercent: 20,
			Message:         "Validating operation inputs",
		})
		inProgressState := "Installing"
		if action == actionRemoveItem {
			inProgressState = "Removing"
		}
		sr.appendOperationEvent(operationID, operationStatusEventPayload{
			State:           inProgressState,
			ProgressPercent: 60,
			Message:         fmt.Sprintf("%s item via managed run", inProgressState),
		})
		if _, err := sr.submit(ctx, Command{Action: actionRun}); err != nil {
			if errors.Is(err, context.Canceled) {
				sr.appendOperationEvent(operationID, operationStatusEventPayload{
					State:           "Canceled",
					ProgressPercent: 60,
					Message:         "Operation canceled",
					CanceledBy:      "service",
				})
				return
			}
			gorillalog.Warn("failed to run managed action after service mutation:", err)
			sr.appendOperationEvent(operationID, operationStatusEventPayload{
				State:           "Failed",
				ProgressPercent: 100,
				Message:         "Operation failed",
				ErrorCode:       "managed_run_failed",
				ErrorMessage:    err.Error(),
			})
			return
		}
		sr.appendOperationEvent(operationID, operationStatusEventPayload{
			State:           "Succeeded",
			ProgressPercent: 100,
			Message:         "Operation completed",
		})
	}()
}

func commandFromRequestEnvelope(req serviceEnvelope[json.RawMessage]) (Command, error) {
	canonicalAction, ok := canonicalizeAction(req.Operation)
	if !ok {
		return Command{}, fmt.Errorf("unsupported service action %q", req.Operation)
	}

	cmd := Command{Action: canonicalAction}
	switch canonicalAction {
	case actionListOptionalInstalls:
		return cmd, nil
	case actionInstallItem:
		payload, err := decodeEnvelopePayload[installItemRequest](req.Payload)
		if err != nil {
			return Command{}, fmt.Errorf("invalid InstallItem payload: %w", err)
		}
		itemName := strings.TrimSpace(payload.ItemName)
		if itemName == "" {
			return Command{}, errors.New("InstallItem requires itemName")
		}
		cmd.Items = []string{itemName}
		return cmd, nil
	case actionRemoveItem:
		payload, err := decodeEnvelopePayload[removeItemRequest](req.Payload)
		if err != nil {
			return Command{}, fmt.Errorf("invalid RemoveItem payload: %w", err)
		}
		itemName := strings.TrimSpace(payload.ItemName)
		if itemName == "" {
			return Command{}, errors.New("RemoveItem requires itemName")
		}
		cmd.Items = []string{itemName}
		return cmd, nil
	case actionStreamOperationStatus:
		operationID := strings.TrimSpace(req.OperationID)
		if operationID == "" {
			return Command{}, errors.New("StreamOperationStatus requires operationId")
		}
		cmd.Items = []string{operationID}
		return cmd, nil
	default:
		return Command{}, fmt.Errorf("unsupported service action %q", req.Operation)
	}
}

func (sr *serviceRunner) writeSuccessEnvelope(file *os.File, req serviceEnvelope[json.RawMessage], cmd Command, resp CommandResponse) error {
	switch cmd.Action {
	case actionListOptionalInstalls:
		items := make([]optionalInstallResponseItem, 0, len(resp.Items))
		sorted := append([]string(nil), resp.Items...)
		slices.Sort(sorted)
		for _, name := range sorted {
			items = append(items, optionalInstallResponseItem{
				ItemName:           name,
				DisplayName:        name,
				Version:            "",
				Catalog:            "",
				InstallerType:      "",
				InstallerPackageID: name,
				InstallerLocation:  "",
				IsManaged:          true,
				IsInstalled:        false,
				Status:             "Unknown",
				StatusUpdatedAtUTC: nowRFC3339UTC(),
			})
		}

		if err := json.NewEncoder(file).Encode(serviceEnvelope[listOptionalInstallsResponse]{
			Version:      pipeProtocolVersion,
			MessageType:  messageTypeResponse,
			Operation:    actionListOptionalInstalls,
			RequestID:    req.RequestID,
			OperationID:  "",
			TimestampUTC: nowRFC3339UTC(),
			Payload:      listOptionalInstallsResponse{Items: items},
		}); err != nil {
			return err
		}
		return nil
	case actionInstallItem, actionRemoveItem:
		if err := json.NewEncoder(file).Encode(serviceEnvelope[operationAcceptedResponse]{
			Version:      pipeProtocolVersion,
			MessageType:  messageTypeResponse,
			Operation:    cmd.Action,
			RequestID:    req.RequestID,
			OperationID:  resp.OperationID,
			TimestampUTC: nowRFC3339UTC(),
			Payload: operationAcceptedResponse{
				Accepted:    true,
				QueuedAtUTC: nowRFC3339UTC(),
			},
		}); err != nil {
			return err
		}
		return nil
	default:
		writeErrorEnvelope(file, req.RequestID, req.Operation, req.OperationID, "unsupported_action", "unsupported service action")
		return nil
	}
}

func (sr *serviceRunner) writeStreamOperationStatusSequence(file *os.File, req serviceEnvelope[json.RawMessage], operationID string) error {
	if !sr.hasTrackedOperation(operationID) {
		writeErrorEnvelope(file, req.RequestID, req.Operation, req.OperationID, "invalid_request", "unknown operationId")
		return nil
	}

	if err := json.NewEncoder(file).Encode(serviceEnvelope[streamOperationStatusAckResponse]{
		Version:      pipeProtocolVersion,
		MessageType:  messageTypeResponse,
		Operation:    actionStreamOperationStatus,
		RequestID:    req.RequestID,
		OperationID:  operationID,
		TimestampUTC: nowRFC3339UTC(),
		Payload: streamOperationStatusAckResponse{
			StreamAccepted: true,
		},
	}); err != nil {
		return err
	}
	gorillalog.Debug("stream ack sent for operationId=", operationID)

	sent := 0
	for {
		events, done, ok := sr.snapshotTrackedOperation(operationID)
		if !ok {
			writeErrorEnvelope(file, req.RequestID, req.Operation, req.OperationID, "invalid_request", "unknown operationId")
			return nil
		}
		for sent < len(events) {
			if err := json.NewEncoder(file).Encode(serviceEnvelope[operationStatusEventPayload]{
				Version:      pipeProtocolVersion,
				MessageType:  messageTypeEvent,
				Operation:    actionStreamOperationStatus,
				RequestID:    "",
				OperationID:  operationID,
				TimestampUTC: nowRFC3339UTC(),
				Payload:      events[sent],
			}); err != nil {
				return err
			}
			sent++
		}
		if done {
			return nil
		}
		time.Sleep(streamPollSleep)
	}
}

func (sr *serviceRunner) registerTrackedOperation(operationID string) {
	if strings.TrimSpace(operationID) == "" {
		return
	}
	sr.operationsMu.Lock()
	defer sr.operationsMu.Unlock()
	sr.pruneTrackedOperationsLocked(time.Now())
	sr.operations[operationID] = &trackedOperation{
		events: []operationStatusEventPayload{
			{
				State:           "Queued",
				ProgressPercent: 0,
				Message:         "Operation queued",
			},
		},
		lastUpdated: time.Now(),
	}
}

func (sr *serviceRunner) appendOperationEvent(operationID string, event operationStatusEventPayload) {
	if strings.TrimSpace(operationID) == "" {
		return
	}
	sr.operationsMu.Lock()
	defer sr.operationsMu.Unlock()
	op, ok := sr.operations[operationID]
	if !ok {
		return
	}
	now := time.Now()
	op.events = append(op.events, event)
	op.lastUpdated = now
	if event.State == "Succeeded" || event.State == "Failed" || event.State == "Canceled" {
		op.done = true
		op.completedAt = now
	}
	sr.pruneTrackedOperationsLocked(now)
}

func (sr *serviceRunner) hasTrackedOperation(operationID string) bool {
	sr.operationsMu.Lock()
	defer sr.operationsMu.Unlock()
	_, ok := sr.operations[operationID]
	return ok
}

func (sr *serviceRunner) snapshotTrackedOperation(operationID string) ([]operationStatusEventPayload, bool, bool) {
	sr.operationsMu.Lock()
	defer sr.operationsMu.Unlock()
	op, ok := sr.operations[operationID]
	if !ok {
		return nil, false, false
	}
	out := make([]operationStatusEventPayload, len(op.events))
	copy(out, op.events)
	return out, op.done, true
}

func (sr *serviceRunner) pruneTrackedOperationsLocked(now time.Time) {
	for id, op := range sr.operations {
		if op.done && !op.completedAt.IsZero() && now.Sub(op.completedAt) > trackedCompletedOperationTTL {
			delete(sr.operations, id)
		}
	}

	if len(sr.operations) <= trackedOperationsMaxCount {
		return
	}

	type doneOp struct {
		id          string
		completedAt time.Time
	}
	done := make([]doneOp, 0, len(sr.operations))
	for id, op := range sr.operations {
		if !op.done {
			continue
		}
		done = append(done, doneOp{id: id, completedAt: op.completedAt})
	}
	sort.Slice(done, func(i, j int) bool {
		return done[i].completedAt.Before(done[j].completedAt)
	})
	for _, candidate := range done {
		if len(sr.operations) <= trackedOperationsMaxCount {
			return
		}
		delete(sr.operations, candidate.id)
	}
}

func createNamedPipe(pipePath string) (windows.Handle, error) {
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GRGW;;;AU)")
	if err != nil {
		return windows.InvalidHandle, fmt.Errorf("security descriptor: %w", err)
	}

	sa := windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd,
		InheritHandle:      0,
	}

	name, err := windows.UTF16PtrFromString(pipePath)
	if err != nil {
		return windows.InvalidHandle, err
	}

	return windows.CreateNamedPipe(
		name,
		windows.PIPE_ACCESS_DUPLEX,
		windows.PIPE_TYPE_MESSAGE|windows.PIPE_READMODE_MESSAGE|windows.PIPE_WAIT,
		windows.PIPE_UNLIMITED_INSTANCES,
		64*1024,
		64*1024,
		0,
		&sa,
	)
}

func (sr *serviceRunner) closeListenerPipe() {
	sr.pipeListenerMu.Lock()
	defer sr.pipeListenerMu.Unlock()
	if sr.pipeListenerHandle != 0 && sr.pipeListenerHandle != windows.InvalidHandle {
		_ = windows.CloseHandle(sr.pipeListenerHandle)
		sr.pipeListenerHandle = 0
	}
}

func (sr *serviceRunner) trackActiveConnection(handle windows.Handle) {
	sr.activeConnMu.Lock()
	defer sr.activeConnMu.Unlock()
	sr.activeConns[handle] = struct{}{}
}

func (sr *serviceRunner) untrackActiveConnection(handle windows.Handle) {
	sr.activeConnMu.Lock()
	defer sr.activeConnMu.Unlock()
	delete(sr.activeConns, handle)
}

func (sr *serviceRunner) closeActiveConnections() {
	sr.activeConnMu.Lock()
	defer sr.activeConnMu.Unlock()
	for handle := range sr.activeConns {
		_ = windows.CloseHandle(handle)
	}
}

func (sr *serviceRunner) clearListenerPipe(handle windows.Handle) {
	sr.pipeListenerMu.Lock()
	defer sr.pipeListenerMu.Unlock()
	if sr.pipeListenerHandle == handle {
		sr.pipeListenerHandle = 0
	}
}

type gorillaWindowsService struct {
	cfg        config.Configuration
	managedRun func(config.Configuration) error
}

func (g *gorillaWindowsService) Execute(_ []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runner := newServiceRunner(g.cfg, g.managedRun)
	if err := runner.start(ctx); err != nil {
		gorillalog.Warn("failed to start service runner:", err)
		return false, 1
	}

	changes <- svc.Status{State: svc.Running, Accepts: accepted}

	for req := range requests {
		switch req.Cmd {
		case svc.Interrogate:
			changes <- req.CurrentStatus
		case svc.Stop, svc.Shutdown:
			changes <- svc.Status{State: svc.StopPending}
			cancel()
			stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
			runner.stop(stopCtx)
			stopCancel()
			return false, 0
		default:
		}
	}

	cancel()
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	runner.stop(stopCtx)
	stopCancel()
	return false, 0
}

func Run(cfg config.Configuration, managedRun func(config.Configuration) error) error {
	return svc.Run(cfg.ServiceName, &gorillaWindowsService{cfg: cfg, managedRun: managedRun})
}
