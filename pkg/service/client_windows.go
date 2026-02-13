//go:build windows

package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/1dustindavis/gorilla/pkg/config"
	"golang.org/x/sys/windows"
)

type pipeRequest struct {
	Command Command `json:"command"`
}

func sendCommand(cfg config.Configuration, cmd Command) (CommandResponse, error) {
	pipePath := servicePipePath(cfg.ServicePipeName)
	conn, err := openPipe(pipePath, 30*time.Second)
	if err != nil {
		return CommandResponse{}, fmt.Errorf("failed to connect to service pipe %s: %w", pipePath, err)
	}
	defer func() {
		_ = conn.Close()
	}()

	req := pipeRequest{Command: cmd}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return CommandResponse{}, fmt.Errorf("failed to send service command: %w", err)
	}

	var resp CommandResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return CommandResponse{}, fmt.Errorf("failed to decode service response: %w", err)
	}
	if resp.Status != "ok" {
		if resp.Message == "" {
			resp.Message = "service command failed"
		}
		return CommandResponse{}, errors.New(resp.Message)
	}

	return resp, nil
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
