package status

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
)

func TestCheckScriptReturnsProcessStartError(t *testing.T) {
	orig := execCommand
	execCommand = func(string, ...string) *exec.Cmd {
		return exec.Command(filepath.Join(t.TempDir(), "missing-powershell.exe"))
	}
	defer func() { execCommand = orig }()

	item := catalog.Item{
		Check: catalog.InstallCheck{Script: "Write-Output check"},
	}

	actionNeeded, err := checkScript(item, t.TempDir(), "install")
	if err == nil {
		t.Fatal("expected process start error")
	}
	if actionNeeded {
		t.Fatal("expected no action decision when script execution cannot start")
	}
}
