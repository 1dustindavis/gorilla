package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/admin"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/service"
)

func resetMainHooks() {
	adminCheckFunc = adminCheck
	mkdirAllFunc = os.MkdirAll
	buildCatalogsFunc = admin.BuildCatalogs
	importItemFunc = admin.ImportItem
	managedRunFunc = managedRun
	runServiceFunc = func(cfg config.Configuration) error { return service.Run(cfg, managedRunFunc) }
	sendServiceCommandFunc = service.SendCommand
	runServiceActionFunc = service.RunAction
}

func TestRunAdminCheckError(t *testing.T) {
	resetMainHooks()
	defer resetMainHooks()

	cfg := config.Configuration{CheckOnly: false}
	adminCheckFunc = func() (bool, error) { return false, errors.New("boom") }
	mkdirAllFunc = func(path string, mode os.FileMode) error {
		t.Fatalf("mkdirAllFunc should not be called when admin check errors")
		return nil
	}

	err := managedRun(cfg)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "unable to check if running as admin: boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRequiresAdmin(t *testing.T) {
	resetMainHooks()
	defer resetMainHooks()

	cfg := config.Configuration{CheckOnly: false}
	adminCheckFunc = func() (bool, error) { return false, nil }
	mkdirAllFunc = func(path string, mode os.FileMode) error {
		t.Fatalf("mkdirAllFunc should not be called when admin check fails")
		return nil
	}

	err := managedRun(cfg)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "requires admnisistrative access") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCheckOnlySkipsAdminCheck(t *testing.T) {
	resetMainHooks()
	defer resetMainHooks()

	cfg := config.Configuration{CheckOnly: true, CachePath: "some/cache/path"}
	adminCalled := false
	adminCheckFunc = func() (bool, error) {
		adminCalled = true
		return false, nil
	}
	mkdirAllFunc = func(path string, mode os.FileMode) error { return errors.New("mkdir failed") }

	err := managedRun(cfg)
	if err == nil {
		t.Fatalf("expected error")
	}
	if adminCalled {
		t.Fatalf("adminCheckFunc should not be called in check-only mode")
	}
	if !strings.Contains(err.Error(), "unable to create cache directory: mkdir failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCreateCacheError(t *testing.T) {
	resetMainHooks()
	defer resetMainHooks()

	cfg := config.Configuration{CheckOnly: false, CachePath: "some/cache/path"}
	adminCheckFunc = func() (bool, error) { return true, nil }
	mkdirAllFunc = func(path string, mode os.FileMode) error { return errors.New("mkdir failed") }

	err := managedRun(cfg)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "unable to create cache directory: mkdir failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunBuildMode(t *testing.T) {
	resetMainHooks()
	defer resetMainHooks()

	cfg := config.Configuration{
		BuildArg:    true,
		CheckOnly:   true,
		RepoPath:    "repo/path",
		CachePath:   "cache/path",
		AppDataPath: t.TempDir(),
	}
	adminCalled := false
	buildCalled := false
	adminCheckFunc = func() (bool, error) {
		adminCalled = true
		return false, nil
	}
	mkdirAllFunc = func(path string, mode os.FileMode) error { return nil }
	buildCatalogsFunc = func(repoPath string) error {
		buildCalled = true
		if repoPath != "repo/path" {
			t.Fatalf("unexpected repoPath: %s", repoPath)
		}
		return nil
	}
	importItemFunc = func(repoPath, itemPath string) error { return nil }

	err := managedRun(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if adminCalled {
		t.Fatalf("adminCheckFunc should not be called in build mode")
	}
	if !buildCalled {
		t.Fatalf("expected buildCatalogsFunc to be called")
	}
}

func TestRunImportModeError(t *testing.T) {
	resetMainHooks()
	defer resetMainHooks()

	cfg := config.Configuration{
		ImportArg:   "x.msi",
		CheckOnly:   true,
		RepoPath:    "repo/path",
		CachePath:   "cache/path",
		AppDataPath: t.TempDir(),
	}
	adminCheckFunc = func() (bool, error) { return true, nil }
	mkdirAllFunc = func(path string, mode os.FileMode) error { return nil }
	buildCatalogsFunc = func(repoPath string) error { return nil }
	importItemFunc = func(repoPath, itemPath string) error {
		if repoPath != "repo/path" || itemPath != "x.msi" {
			t.Fatalf("unexpected args repo=%s item=%s", repoPath, itemPath)
		}
		return errors.New("not implemented")
	}

	err := managedRun(cfg)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "error importing item: not implemented") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteServiceModesSkipRun(t *testing.T) {
	resetMainHooks()
	defer resetMainHooks()

	serviceAction := ""
	serviceCommand := ""
	serviceMode := false
	runCalled := false

	managedRunFunc = func(cfg config.Configuration) error {
		runCalled = true
		return nil
	}
	runServiceActionFunc = func(cfg config.Configuration, action string) error {
		serviceAction = action
		return nil
	}
	sendServiceCommandFunc = func(cfg config.Configuration, spec string) (service.CommandResponse, error) {
		serviceCommand = spec
		return service.CommandResponse{Status: "ok"}, nil
	}
	runServiceFunc = func(cfg config.Configuration) error {
		serviceMode = true
		return nil
	}

	tests := []struct {
		name          string
		cfg           config.Configuration
		wantAction    string
		wantCommand   string
		wantSvcMode   bool
		expectRunCall bool
	}{
		{
			name: "service install",
			cfg: config.Configuration{
				ServiceInstall: true,
			},
			wantAction:    "install",
			expectRunCall: false,
		},
		{
			name: "service remove",
			cfg: config.Configuration{
				ServiceRemove: true,
			},
			wantAction:    "remove",
			expectRunCall: false,
		},
		{
			name: "service start",
			cfg: config.Configuration{
				ServiceStart: true,
			},
			wantAction:    "start",
			expectRunCall: false,
		},
		{
			name: "service stop",
			cfg: config.Configuration{
				ServiceStop: true,
			},
			wantAction:    "stop",
			expectRunCall: false,
		},
		{
			name: "service command",
			cfg: config.Configuration{
				ServiceCommand: "run",
			},
			wantCommand:   "run",
			expectRunCall: false,
		},
		{
			name: "service mode",
			cfg: config.Configuration{
				ServiceMode: true,
			},
			wantSvcMode:   true,
			expectRunCall: false,
		},
		{
			name:          "normal mode",
			cfg:           config.Configuration{},
			expectRunCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serviceAction = ""
			serviceCommand = ""
			serviceMode = false
			runCalled = false

			err := route(tt.cfg)
			if err != nil {
				t.Fatalf("execute returned unexpected error: %v", err)
			}

			if runCalled != tt.expectRunCall {
				t.Fatalf("managedRun called = %v, expected %v", runCalled, tt.expectRunCall)
			}
			if serviceAction != tt.wantAction {
				t.Fatalf("service action = %q, expected %q", serviceAction, tt.wantAction)
			}
			if serviceCommand != tt.wantCommand {
				t.Fatalf("service command = %q, expected %q", serviceCommand, tt.wantCommand)
			}
			if serviceMode != tt.wantSvcMode {
				t.Fatalf("service mode call = %v, expected %v", serviceMode, tt.wantSvcMode)
			}
		})
	}
}

func TestRoutePrecedenceServiceInstallWins(t *testing.T) {
	resetMainHooks()
	defer resetMainHooks()

	serviceAction := ""
	serviceCommandCalled := false
	serviceModeCalled := false
	runCalled := false

	runServiceActionFunc = func(cfg config.Configuration, action string) error {
		serviceAction = action
		return nil
	}
	sendServiceCommandFunc = func(cfg config.Configuration, spec string) (service.CommandResponse, error) {
		serviceCommandCalled = true
		return service.CommandResponse{Status: "ok"}, nil
	}
	runServiceFunc = func(cfg config.Configuration) error {
		serviceModeCalled = true
		return nil
	}
	managedRunFunc = func(cfg config.Configuration) error {
		runCalled = true
		return nil
	}

	cfg := config.Configuration{
		ServiceInstall: true,
		ServiceRemove:  true,
		ServiceStart:   true,
		ServiceStop:    true,
		ServiceCommand: "run",
		ServiceMode:    true,
	}
	if err := route(cfg); err != nil {
		t.Fatalf("unexpected route error: %v", err)
	}

	if serviceAction != "install" {
		t.Fatalf("expected install action, got %q", serviceAction)
	}
	if serviceCommandCalled {
		t.Fatalf("service command branch should not run when service install is set")
	}
	if serviceModeCalled {
		t.Fatalf("service mode branch should not run when service install is set")
	}
	if runCalled {
		t.Fatalf("managedRun should not run when service install is set")
	}
}

func TestRouteServiceCommandPrintsItems(t *testing.T) {
	resetMainHooks()
	defer resetMainHooks()

	sendServiceCommandFunc = func(cfg config.Configuration, spec string) (service.CommandResponse, error) {
		return service.CommandResponse{
			Status: "ok",
			Items:  []string{"GoogleChrome", "VSCode"},
		}, nil
	}

	stdout := captureStdout(t, func() {
		err := route(config.Configuration{ServiceCommand: "get-service-manifest"})
		if err != nil {
			t.Fatalf("unexpected route error: %v", err)
		}
	})

	if !strings.Contains(stdout, "GoogleChrome") || !strings.Contains(stdout, "VSCode") {
		t.Fatalf("expected stdout to include response items, got %q", stdout)
	}
}

func TestRouteServiceCommandErrorDoesNotPrintItems(t *testing.T) {
	resetMainHooks()
	defer resetMainHooks()

	sendServiceCommandFunc = func(cfg config.Configuration, spec string) (service.CommandResponse, error) {
		return service.CommandResponse{}, errors.New("boom")
	}

	stdout := captureStdout(t, func() {
		err := route(config.Configuration{ServiceCommand: "get-service-manifest"})
		if err == nil {
			t.Fatalf("expected route error")
		}
		if !strings.Contains(err.Error(), "boom") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("expected no stdout output on command error, got %q", stdout)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe creation failed: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = origStdout
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("stdout pipe close failed: %v", err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("stdout pipe read failed: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("stdout pipe close failed: %v", err)
	}

	return fmt.Sprint(buf.String())
}
