//go:build windows

package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/1dustindavis/gorilla/pkg/config"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func serviceVerboseEnabled(cfg config.Configuration) bool {
	return cfg.Verbose || cfg.Debug
}

func serviceVerbose(cfg config.Configuration, format string, args ...interface{}) {
	if !serviceVerboseEnabled(cfg) {
		return
	}
	fmt.Printf("[service] "+format+"\n", args...)
}

func RunAction(cfg config.Configuration, action string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer m.Disconnect()
	serviceVerbose(cfg, "action=%s service=%q", action, cfg.ServiceName)

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

func ServiceStatus(cfg config.Configuration) (string, error) {
	m, err := mgr.Connect()
	if err != nil {
		return "", fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(cfg.ServiceName)
	if err != nil {
		return "", fmt.Errorf("failed to open service %q: %w", cfg.ServiceName, err)
	}
	defer s.Close()

	status, err := s.Query()
	if err != nil {
		return "", fmt.Errorf("failed to query service %q: %w", cfg.ServiceName, err)
	}
	serviceConfig, err := s.Config()
	if err != nil {
		return "", fmt.Errorf("failed to read service config for %q: %w", cfg.ServiceName, err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "name: %s\n", cfg.ServiceName)
	fmt.Fprintf(&b, "display_name: %s\n", serviceConfig.DisplayName)
	fmt.Fprintf(&b, "state: %s\n", serviceStateString(status.State))
	fmt.Fprintf(&b, "start_type: %s\n", serviceStartTypeString(serviceConfig.StartType))
	fmt.Fprintf(&b, "start_account: %s\n", serviceConfig.ServiceStartName)
	fmt.Fprintf(&b, "binary_path: %s\n", serviceConfig.BinaryPathName)
	fmt.Fprintf(&b, "description: %s\n", serviceConfig.Description)
	fmt.Fprintf(&b, "accepts: %s\n", serviceAcceptsString(status.Accepts))

	return strings.TrimSpace(b.String()), nil
}

func serviceStateString(state svc.State) string {
	switch state {
	case svc.Stopped:
		return "stopped"
	case svc.StartPending:
		return "start_pending"
	case svc.StopPending:
		return "stop_pending"
	case svc.Running:
		return "running"
	case svc.ContinuePending:
		return "continue_pending"
	case svc.PausePending:
		return "pause_pending"
	case svc.Paused:
		return "paused"
	default:
		return fmt.Sprintf("unknown(%d)", state)
	}
}

func serviceStartTypeString(startType uint32) string {
	switch startType {
	case mgr.StartAutomatic:
		return "automatic"
	case mgr.StartManual:
		return "manual"
	case mgr.StartDisabled:
		return "disabled"
	default:
		return fmt.Sprintf("unknown(%d)", startType)
	}
}

func serviceAcceptsString(accepts svc.Accepted) string {
	parts := make([]string, 0, 5)
	if accepts&svc.AcceptStop != 0 {
		parts = append(parts, "stop")
	}
	if accepts&svc.AcceptShutdown != 0 {
		parts = append(parts, "shutdown")
	}
	if accepts&svc.AcceptPauseAndContinue != 0 {
		parts = append(parts, "pause_continue")
	}
	if accepts&svc.AcceptParamChange != 0 {
		parts = append(parts, "param_change")
	}
	if accepts&svc.AcceptNetBindChange != 0 {
		parts = append(parts, "net_bind_change")
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ",")
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
	startArgs := serviceInstallArgs(configPath)
	serviceVerbose(cfg, "install config: exe=%q config=%q args=%q start_type=automatic account=%q", exePath, configPath, startArgs, "LocalSystem")

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
		startArgs...,
	)
	if err != nil {
		return fmt.Errorf("failed to create service %q: %w", cfg.ServiceName, err)
	}
	defer s.Close()
	serviceVerbose(cfg, "install complete for service=%q", cfg.ServiceName)

	return nil
}

func startWindowsService(m *mgr.Mgr, cfg config.Configuration) error {
	serviceVerbose(cfg, "start requested for service=%q", cfg.ServiceName)
	s, err := m.OpenService(cfg.ServiceName)
	if err != nil {
		return fmt.Errorf("failed to open service %q: %w", cfg.ServiceName, err)
	}
	defer s.Close()

	status, err := s.Query()
	if err == nil && status.State == svc.Running {
		serviceVerbose(cfg, "service=%q already running", cfg.ServiceName)
		return nil
	}

	if err := s.Start(); err != nil {
		return fmt.Errorf("failed to start service %q: %w", cfg.ServiceName, err)
	}
	serviceVerbose(cfg, "start command accepted for service=%q", cfg.ServiceName)
	return nil
}

func stopWindowsService(m *mgr.Mgr, cfg config.Configuration) error {
	serviceVerbose(cfg, "stop requested for service=%q", cfg.ServiceName)
	s, err := m.OpenService(cfg.ServiceName)
	if err != nil {
		return fmt.Errorf("failed to open service %q: %w", cfg.ServiceName, err)
	}
	defer s.Close()

	status, err := s.Control(svc.Stop)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
			serviceVerbose(cfg, "service=%q already stopped", cfg.ServiceName)
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
	serviceVerbose(cfg, "service=%q stopped", cfg.ServiceName)

	return nil
}

func removeWindowsService(m *mgr.Mgr, cfg config.Configuration) error {
	serviceVerbose(cfg, "remove requested for service=%q", cfg.ServiceName)
	s, err := m.OpenService(cfg.ServiceName)
	if err != nil {
		return fmt.Errorf("failed to open service %q: %w", cfg.ServiceName, err)
	}

	if status, err := s.Query(); err == nil && status.State != svc.Stopped {
		serviceVerbose(cfg, "service=%q not stopped; issuing stop before delete", cfg.ServiceName)
		_, _ = s.Control(svc.Stop)
		time.Sleep(500 * time.Millisecond)
	}

	if err := s.Delete(); err != nil {
		_ = s.Close()
		return fmt.Errorf("failed to remove service %q: %w", cfg.ServiceName, err)
	}
	serviceVerbose(cfg, "delete issued for service=%q", cfg.ServiceName)

	// Delete marks a service for deletion; it may remain visible while other
	// processes (for example Services MMC) still hold an open handle.
	if err := s.Close(); err != nil {
		return fmt.Errorf("failed closing service handle for %q: %w", cfg.ServiceName, err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		existing, err := m.OpenService(cfg.ServiceName)
		if err != nil {
			if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
				serviceVerbose(cfg, "service=%q fully removed", cfg.ServiceName)
				return nil
			}
			return fmt.Errorf("failed to verify removal of service %q: %w", cfg.ServiceName, err)
		}
		_ = existing.Close()

		if time.Now().After(deadline) {
			return fmt.Errorf("service %q is marked for deletion but still present; close Services and try again", cfg.ServiceName)
		}
		time.Sleep(250 * time.Millisecond)
	}
}
