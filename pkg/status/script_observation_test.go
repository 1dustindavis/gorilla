package status

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
)

func TestObserveScriptState(t *testing.T) {
	execCommand = fakeExecCommand
	defer func() { execCommand = origExec }()

	tests := []struct {
		name             string
		installType      string
		marker           string
		wantState        ObservedState
		wantActionNeeded bool
		wantDetail       string
	}{
		{
			name:             "install action needed",
			installType:      "install",
			marker:           statusActionNoError,
			wantState:        Absent,
			wantActionNeeded: true,
			wantDetail:       "script_requirement_not_satisfied",
		},
		{
			name:             "install action not needed",
			installType:      "install",
			marker:           statusNoActionNoError,
			wantState:        Installed,
			wantActionNeeded: false,
			wantDetail:       "script_requirement_satisfied",
		},
		{
			name:             "update action needed",
			installType:      "update",
			marker:           statusActionNoError,
			wantState:        Absent,
			wantActionNeeded: true,
			wantDetail:       "script_requirement_not_satisfied",
		},
		{
			name:             "update action not needed",
			installType:      "update",
			marker:           statusNoActionNoError,
			wantState:        Installed,
			wantActionNeeded: false,
			wantDetail:       "script_requirement_satisfied",
		},
		{
			name:             "uninstall action needed",
			installType:      "uninstall",
			marker:           statusNoActionNoError,
			wantState:        Installed,
			wantActionNeeded: true,
			wantDetail:       "script_requirement_not_satisfied",
		},
		{
			name:             "uninstall action not needed",
			installType:      "uninstall",
			marker:           statusActionNoError,
			wantState:        Absent,
			wantActionNeeded: false,
			wantDetail:       "script_requirement_satisfied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := catalog.Item{
				Check: catalog.InstallCheck{Script: tt.marker},
			}

			got, err := Observe(item, tt.installType, t.TempDir())
			if err != nil {
				t.Fatalf("Observe returned error: %v", err)
			}
			if got.State != tt.wantState {
				t.Errorf("State = %q, want %q", got.State, tt.wantState)
			}
			if got.ActionNeeded != tt.wantActionNeeded {
				t.Errorf("ActionNeeded = %v, want %v", got.ActionNeeded, tt.wantActionNeeded)
			}
			if got.DetailCode != tt.wantDetail {
				t.Errorf("DetailCode = %q, want %q", got.DetailCode, tt.wantDetail)
			}
			if got.InstalledVersion != "" {
				t.Errorf("InstalledVersion = %q, want empty", got.InstalledVersion)
			}
		})
	}
}

func TestObserveScriptExecutionFailure(t *testing.T) {
	orig := execCommand
	execCommand = func(string, ...string) *exec.Cmd {
		return exec.Command(filepath.Join(t.TempDir(), "missing-powershell.exe"))
	}
	defer func() { execCommand = orig }()

	item := catalog.Item{
		Check: catalog.InstallCheck{Script: "Write-Output check"},
	}

	got, err := Observe(item, "install", t.TempDir())
	if err == nil {
		t.Fatal("expected process start error")
	}
	if got.State != DetectionFailed {
		t.Errorf("State = %q, want %q", got.State, DetectionFailed)
	}
	if got.ActionNeeded {
		t.Error("ActionNeeded = true, want false when script execution cannot start")
	}
	if got.DetailCode != "check_failed" {
		t.Errorf("DetailCode = %q, want %q", got.DetailCode, "check_failed")
	}
	if got.InstalledVersion != "" {
		t.Errorf("InstalledVersion = %q, want empty", got.InstalledVersion)
	}
}
