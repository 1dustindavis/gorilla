package installer

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/download"
	"github.com/1dustindavis/gorilla/pkg/report"
)

func TestInstallReturnsInstallerCommandError(t *testing.T) {
	origStatus := statusCheckStatus
	origRunner := runCommand
	origInstall := installItemFunc
	origExecCommand := execCommand
	origInstalledItems := report.InstalledItems
	defer func() {
		statusCheckStatus = origStatus
		runCommand = origRunner
		installItemFunc = origInstall
		execCommand = origExecCommand
		report.InstalledItems = origInstalledItems
	}()

	download.SetConfig(downloadCfg)
	statusCheckStatus = func(catalog.Item, string, string) (bool, error) { return true, nil }
	runnerErr := errors.New("deliberate installer failure")
	runCommand = func(string, []string) (string, error) { return "runner output", runnerErr }
	installItemFunc = installItemResult
	report.InstalledItems = nil

	postInstallRan := false
	execCommand = func(command string, args ...string) *exec.Cmd {
		postInstallRan = true
		return fakeExecCommand(command, args...)
	}

	item := msiItem
	item.DisplayName = "Installer Failure"
	item.PostScript = "Write-Output post"

	got := Install(item, "install", "https://example.com/", "testdata/", false)
	want := "Installation error: deliberate installer failure"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
	if postInstallRan {
		t.Fatal("post-install script ran after installer command failure")
	}
	if len(report.InstalledItems) != 1 {
		t.Fatalf("expected failed installation attempt in report, got %d items", len(report.InstalledItems))
	}
}

func TestInstallReturnsUninstallerCommandError(t *testing.T) {
	origStatus := statusCheckStatus
	origRunner := runCommand
	origUninstall := uninstallItemFunc
	origUninstalledItems := report.UninstalledItems
	defer func() {
		statusCheckStatus = origStatus
		runCommand = origRunner
		uninstallItemFunc = origUninstall
		report.UninstalledItems = origUninstalledItems
	}()

	download.SetConfig(downloadCfg)
	statusCheckStatus = func(catalog.Item, string, string) (bool, error) { return true, nil }
	runnerErr := errors.New("deliberate uninstaller failure")
	runCommand = func(string, []string) (string, error) { return "runner output", runnerErr }
	uninstallItemFunc = uninstallItemResult
	report.UninstalledItems = nil

	item := msiItem
	item.DisplayName = "Uninstaller Failure"

	got := Install(item, "uninstall", "https://example.com/", "testdata/", false)
	want := "Uninstallation error: deliberate uninstaller failure"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
	if len(report.UninstalledItems) != 1 {
		t.Fatalf("expected failed uninstallation attempt in report, got %d items", len(report.UninstalledItems))
	}
}
