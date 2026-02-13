//go:build windows

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
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
	wg                 sync.WaitGroup
	execMutex          sync.Mutex
	pipeListenerMu     sync.Mutex
	pipeListenerHandle windows.Handle
}

func newServiceRunner(cfg config.Configuration, managedRun func(config.Configuration) error) *serviceRunner {
	return &serviceRunner{
		cfg:        cfg,
		managedRun: managedRun,
		queue:      make(chan queuedCommand),
	}
}

func (sr *serviceRunner) start(ctx context.Context) error {
	gorillalog.NewLog(sr.cfg)

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
				resp, err := executeCommand(sr.cfg, queued.cmd, sr.managedRun)
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
			gorillalog.Error("service named pipe endpoint failed:", err)
		}
	}()

	return nil
}

func (sr *serviceRunner) stop(ctx context.Context) {
	sr.closeListenerPipe()
	sr.wg.Wait()
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

func writeCommandResponse(file *os.File, status, message string) {
	_ = json.NewEncoder(file).Encode(CommandResponse{
		Status:  status,
		Message: message,
	})
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

		file := os.NewFile(uintptr(handle), pipePath)
		sr.handlePipeCommand(ctx, file)
		_ = windows.DisconnectNamedPipe(handle)
		_ = file.Close()
	}
}

func (sr *serviceRunner) handlePipeCommand(ctx context.Context, file *os.File) {
	var req pipeRequest
	if err := json.NewDecoder(file).Decode(&req); err != nil {
		writeCommandResponse(file, "error", "invalid JSON request body")
		return
	}

	req.Command.Action = strings.ToLower(strings.TrimSpace(req.Command.Action))
	if err := validateCommand(req.Command); err != nil {
		writeCommandResponse(file, "error", err.Error())
		return
	}

	resp, err := sr.submit(ctx, req.Command)
	if err != nil {
		writeCommandResponse(file, "error", err.Error())
		return
	}

	_ = json.NewEncoder(file).Encode(resp)
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
		gorillalog.Error("failed to start service runner:", err)
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
