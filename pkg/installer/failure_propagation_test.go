package installer

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/download"
	"github.com/1dustindavis/gorilla/pkg/report"
)

func failingCommandRunner(runnerErr error) commandRunner {
	return func(command string, arguments []string) (string, error) {
		if command == commandNupkg && len(arguments) > 0 && arguments[0] == "list" {
			return "chef-client", nil
		}
		return "runner output", runnerErr
	}
}

func TestInstallerCommandErrorsPropagateForAllTypes(t *testing.T) {
	download.SetConfig(downloadCfg)
	runnerErr := errors.New("deliberate installer failure")

	tests := []struct {
		name string
		item catalog.Item
		url  string
	}{
		{"nupkg", nupkgItem, "https://example.com/chef-client/chef-client-14.3.37-1-x64.nupkg"},
		{"msi", msiItem, "https://example.com/chef-client/chef-client-14.3.37-1-x64.msi"},
		{"exe", exeItem, "https://example.com/chef-client/chef-client-14.3.37-1-x64.exe"},
		{"ps1", ps1Item, "https://example.com/chef-client/chef-client-14.3.37-1-x64.ps1"},
		{"msix", msixItem, "https://example.com/chef-client/chef-client-14.3.37-1-x64.msix"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origInstalledItems := report.InstalledItems
			defer func() { report.InstalledItems = origInstalledItems }()
			report.InstalledItems = nil

			output, err := installItemResultWithRunner(tt.item, tt.url, "testdata/", failingCommandRunner(runnerErr))
			if !errors.Is(err, runnerErr) {
				t.Fatalf("expected runner error, got %v", err)
			}
			if output != "runner output" {
				t.Fatalf("expected runner output, got %q", output)
			}
			if len(report.InstalledItems) != 1 {
				t.Fatalf("expected failed installation attempt in report, got %d items", len(report.InstalledItems))
			}
		})
	}
}

func TestUninstallerCommandErrorsPropagateForAllTypes(t *testing.T) {
	download.SetConfig(downloadCfg)
	runnerErr := errors.New("deliberate uninstaller failure")

	tests := []struct {
		name string
		item catalog.Item
		url  string
	}{
		{"nupkg", nupkgItem, "https://example.com/chef-client/chef-client-14.3.37-1-x64uninst.nupkg"},
		{"msi", msiItem, "https://example.com/chef-client/chef-client-14.3.37-1-x64uninst.msi"},
		{"exe", exeItem, "https://example.com/chef-client/chef-client-14.3.37-1-x64uninst.exe"},
		{"ps1", ps1Item, "https://example.com/chef-client/chef-client-14.3.37-1-x64uninst.ps1"},
		{"msix", msixItem, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origUninstalledItems := report.UninstalledItems
			defer func() { report.UninstalledItems = origUninstalledItems }()
			report.UninstalledItems = nil

			output, err := uninstallItemResultWithRunner(tt.item, tt.url, "testdata/", failingCommandRunner(runnerErr))
			if !errors.Is(err, runnerErr) {
				t.Fatalf("expected runner error, got %v", err)
			}
			if output != "runner output" {
				t.Fatalf("expected runner output, got %q", output)
			}
			if len(report.UninstalledItems) != 1 {
				t.Fatalf("expected failed uninstallation attempt in report, got %d items", len(report.UninstalledItems))
			}
		})
	}
}

func TestNupkgLookupErrorsPropagateBeforeActionAttempt(t *testing.T) {
	download.SetConfig(downloadCfg)
	lookupErr := errors.New("deliberate nupkg lookup failure")
	runner := func(string, []string) (string, error) { return "", lookupErr }

	origInstalledItems := report.InstalledItems
	origUninstalledItems := report.UninstalledItems
	defer func() {
		report.InstalledItems = origInstalledItems
		report.UninstalledItems = origUninstalledItems
	}()
	report.InstalledItems = nil
	report.UninstalledItems = nil

	installOutput, err := installItemResultWithRunner(nupkgItem, "https://example.com/chef-client/chef-client-14.3.37-1-x64.nupkg", "testdata/", runner)
	if !errors.Is(err, lookupErr) {
		t.Fatalf("expected install lookup error, got %v", err)
	}
	if !strings.Contains(installOutput, "Unable to determine nupkg id") {
		t.Fatalf("expected nupkg lookup failure output, got %q", installOutput)
	}
	if len(report.InstalledItems) != 0 {
		t.Fatalf("expected no install attempt in report, got %d items", len(report.InstalledItems))
	}

	uninstallOutput, err := uninstallItemResultWithRunner(nupkgItem, "https://example.com/chef-client/chef-client-14.3.37-1-x64uninst.nupkg", "testdata/", runner)
	if !errors.Is(err, lookupErr) {
		t.Fatalf("expected uninstall lookup error, got %v", err)
	}
	if !strings.Contains(uninstallOutput, "Unable to determine nupkg id") {
		t.Fatalf("expected nupkg lookup failure output, got %q", uninstallOutput)
	}
	if len(report.UninstalledItems) != 0 {
		t.Fatalf("expected no uninstall attempt in report, got %d items", len(report.UninstalledItems))
	}
}

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
