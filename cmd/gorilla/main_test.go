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
	"github.com/1dustindavis/gorilla/pkg/report"
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
	serviceStatusFunc = service.ServiceStatus
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

func TestManagedRunFinalizesReportOnManifestError(t *testing.T) {
	resetMainHooks()
	defer resetMainHooks()

	cfg := config.Configuration{
		CheckOnly: false,
		CachePath: t.TempDir(),
		URL:       "http://127.0.0.1:1/",
		Manifest:  "missing-manifest",
	}

	report.Items = make(map[string]interface{})
	report.InstalledItems = nil
	report.UninstalledItems = nil
	t.Cleanup(func() {
		report.Items = make(map[string]interface{})
		report.InstalledItems = nil
		report.UninstalledItems = nil
	})

	adminCheckFunc = func() (bool, error) { return true, nil }
	mkdirAllFunc = func(path string, mode os.FileMode) error { return nil }

	err := managedRun(cfg)
	if err == nil {
		t.Fatalf("expected error from manifest retrieval")
	}

	if _, ok := report.Items["EndTime"]; !ok {
		t.Fatalf("expected report EndTime to be set on manifest retrieval failure")
	}
}

func TestExecuteServiceModesSkipRun(t *testing.T) {
	resetMainHooks()
	defer resetMainHooks()

	serviceAction := ""
	serviceCommand := ""
	serviceMode := false
	serviceStatusCalled := false
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
	serviceStatusFunc = func(cfg config.Configuration) (string, error) {
		serviceStatusCalled = true
		return "running", nil
	}

	tests := []struct {
		name          string
		cfg           config.Configuration
		wantAction    string
		wantCommand   string
		wantSvcMode   bool
		wantSvcStatus bool
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
			name: "service status",
			cfg: config.Configuration{
				ServiceStatus: true,
			},
			wantSvcStatus: true,
			expectRunCall: false,
		},
		{
			name: "service command",
			cfg: config.Configuration{
				ServiceCommand: "ListOptionalInstalls",
			},
			wantCommand:   "ListOptionalInstalls",
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
			serviceStatusCalled = false
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
			if serviceStatusCalled != tt.wantSvcStatus {
				t.Fatalf("service status call = %v, expected %v", serviceStatusCalled, tt.wantSvcStatus)
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
	serviceStatusCalled := false
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
	serviceStatusFunc = func(cfg config.Configuration) (string, error) {
		serviceStatusCalled = true
		return "running", nil
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
		ServiceStatus:  true,
		ServiceCommand: "ListOptionalInstalls",
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
	if serviceStatusCalled {
		t.Fatalf("service status branch should not run when service install is set")
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
		err := route(config.Configuration{ServiceCommand: "ListOptionalInstalls"})
		if err != nil {
			t.Fatalf("unexpected route error: %v", err)
		}
	})

	if !strings.Contains(stdout, "GoogleChrome") || !strings.Contains(stdout, "VSCode") {
		t.Fatalf("expected stdout to include response items, got %q", stdout)
	}
}

func TestRouteServiceStatusPrintsValue(t *testing.T) {
	resetMainHooks()
	defer resetMainHooks()

	serviceStatusFunc = func(cfg config.Configuration) (string, error) {
		return "state: running\nstart_type: automatic", nil
	}

	stdout := captureStdout(t, func() {
		err := route(config.Configuration{ServiceStatus: true})
		if err != nil {
			t.Fatalf("unexpected route error: %v", err)
		}
	})

	if !strings.Contains(stdout, "Service status:") {
		t.Fatalf("expected stdout to include service status header, got %q", stdout)
	}
	if !strings.Contains(stdout, "state: running") {
		t.Fatalf("expected stdout to include service status, got %q", stdout)
	}
}

func TestRouteServiceActionPrintsSuccess(t *testing.T) {
	resetMainHooks()
	defer resetMainHooks()

	runServiceActionFunc = func(cfg config.Configuration, action string) error {
		return nil
	}

	tests := []struct {
		name       string
		cfg        config.Configuration
		wantOutput string
	}{
		{
			name: "service install",
			cfg: config.Configuration{
				ServiceInstall: true,
			},
			wantOutput: "Service installed successfully",
		},
		{
			name: "service remove",
			cfg: config.Configuration{
				ServiceRemove: true,
			},
			wantOutput: "Service removed successfully",
		},
		{
			name: "service start",
			cfg: config.Configuration{
				ServiceStart: true,
			},
			wantOutput: "Service started successfully",
		},
		{
			name: "service stop",
			cfg: config.Configuration{
				ServiceStop: true,
			},
			wantOutput: "Service stopped successfully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := captureStdout(t, func() {
				err := route(tt.cfg)
				if err != nil {
					t.Fatalf("unexpected route error: %v", err)
				}
			})

			if !strings.Contains(stdout, tt.wantOutput) {
				t.Fatalf("expected stdout to include %q, got %q", tt.wantOutput, stdout)
			}
		})
	}
}

func TestRouteServiceCommandPrintsSuccessWhenNoItems(t *testing.T) {
	resetMainHooks()
	defer resetMainHooks()

	sendServiceCommandFunc = func(cfg config.Configuration, spec string) (service.CommandResponse, error) {
		return service.CommandResponse{Status: "ok"}, nil
	}

	stdout := captureStdout(t, func() {
		err := route(config.Configuration{ServiceCommand: "InstallItem:GoogleChrome"})
		if err != nil {
			t.Fatalf("unexpected route error: %v", err)
		}
	})

	if !strings.Contains(stdout, "InstallItem command completed successfully") {
		t.Fatalf("expected stdout to include success message, got %q", stdout)
	}
}

func TestRouteServiceCommandPrintsNoneForEmptyListOptionalInstalls(t *testing.T) {
	resetMainHooks()
	defer resetMainHooks()

	sendServiceCommandFunc = func(cfg config.Configuration, spec string) (service.CommandResponse, error) {
		return service.CommandResponse{Status: "ok"}, nil
	}

	stdout := captureStdout(t, func() {
		err := route(config.Configuration{ServiceCommand: "ListOptionalInstalls"})
		if err != nil {
			t.Fatalf("unexpected route error: %v", err)
		}
	})

	if strings.TrimSpace(stdout) != "none" {
		t.Fatalf("expected stdout to be none, got %q", stdout)
	}
}

func TestRouteServiceCommandErrorDoesNotPrintItems(t *testing.T) {
	resetMainHooks()
	defer resetMainHooks()

	sendServiceCommandFunc = func(cfg config.Configuration, spec string) (service.CommandResponse, error) {
		return service.CommandResponse{}, errors.New("boom")
	}

	stdout := captureStdout(t, func() {
		err := route(config.Configuration{ServiceCommand: "ListOptionalInstalls"})
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
