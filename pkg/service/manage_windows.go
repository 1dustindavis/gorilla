//go:build windows

package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/1dustindavis/gorilla/pkg/config"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func RunAction(cfg config.Configuration, action string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer m.Disconnect()

	switch action {
	case "install":
		return installWindowsService(m, cfg)
	case "remove":
		return removeWindowsService(m, cfg)
	case "start":
		return startWindowsService(m, cfg)
	case "stop":
		return stopWindowsService(m, cfg)
	default:
		return fmt.Errorf("unsupported service action %q", action)
	}
}

func installWindowsService(m *mgr.Mgr, cfg config.Configuration) error {
	if cfg.ConfigPath == "" {
		return errors.New("config path is required to install service")
	}
	configPath, err := filepath.Abs(cfg.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to resolve config path: %w", err)
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}
	exePath = filepath.Clean(exePath)

	if existing, err := m.OpenService(cfg.ServiceName); err == nil {
		existing.Close()
		return fmt.Errorf("service %q already exists", cfg.ServiceName)
	} else if !errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return fmt.Errorf("failed to query existing service %q: %w", cfg.ServiceName, err)
	}

	s, err := m.CreateService(
		cfg.ServiceName,
		exePath,
		mgr.Config{
			DisplayName:      "Gorilla",
			Description:      "Gorilla application management service",
			StartType:        mgr.StartAutomatic,
			ServiceStartName: "LocalSystem",
		},
		fmt.Sprintf(`-c "%s" -service`, configPath),
	)
	if err != nil {
		return fmt.Errorf("failed to create service %q: %w", cfg.ServiceName, err)
	}
	defer s.Close()

	return nil
}

func startWindowsService(m *mgr.Mgr, cfg config.Configuration) error {
	s, err := m.OpenService(cfg.ServiceName)
	if err != nil {
		return fmt.Errorf("failed to open service %q: %w", cfg.ServiceName, err)
	}
	defer s.Close()

	status, err := s.Query()
	if err == nil && status.State == svc.Running {
		return nil
	}

	if err := s.Start(); err != nil {
		return fmt.Errorf("failed to start service %q: %w", cfg.ServiceName, err)
	}
	return nil
}

func stopWindowsService(m *mgr.Mgr, cfg config.Configuration) error {
	s, err := m.OpenService(cfg.ServiceName)
	if err != nil {
		return fmt.Errorf("failed to open service %q: %w", cfg.ServiceName, err)
	}
	defer s.Close()

	status, err := s.Control(svc.Stop)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
			return nil
		}
		return fmt.Errorf("failed to stop service %q: %w", cfg.ServiceName, err)
	}

	deadline := time.Now().Add(15 * time.Second)
	for status.State != svc.Stopped {
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for service %q to stop", cfg.ServiceName)
		}
		time.Sleep(300 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			return fmt.Errorf("failed querying service %q: %w", cfg.ServiceName, err)
		}
	}

	return nil
}

func removeWindowsService(m *mgr.Mgr, cfg config.Configuration) error {
	s, err := m.OpenService(cfg.ServiceName)
	if err != nil {
		return fmt.Errorf("failed to open service %q: %w", cfg.ServiceName, err)
	}
	defer s.Close()

	if status, err := s.Query(); err == nil && status.State != svc.Stopped {
		_, _ = s.Control(svc.Stop)
		time.Sleep(500 * time.Millisecond)
	}

	if err := s.Delete(); err != nil {
		return fmt.Errorf("failed to remove service %q: %w", cfg.ServiceName, err)
	}

	return nil
}
