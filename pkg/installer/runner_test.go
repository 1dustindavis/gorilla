package installer

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/report"
)

func TestGetNupkgIDsWithRunner(t *testing.T) {
	var gotCommand string
	var gotArguments []string
	runner := func(command string, arguments []string) (string, error) {
		gotCommand = command
		gotArguments = append([]string(nil), arguments...)
		return " chef-client \n\nchef-client-alt\n", nil
	}

	ids, err := getNupkgIDsWithRunner(`C:\packages`, "--version=1.2.3", runner)
	if err != nil {
		t.Fatalf("getNupkgIDsWithRunner returned error: %v", err)
	}
	if want := []string{"chef-client", "chef-client-alt"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %#v, want %#v", ids, want)
	}
	if gotCommand != commandNupkg {
		t.Fatalf("command = %q, want %q", gotCommand, commandNupkg)
	}
	wantArguments := []string{"list", "--version=1.2.3", "--id-only", "-r", "-s", `C:\packages`}
	if !reflect.DeepEqual(gotArguments, wantArguments) {
		t.Fatalf("arguments = %#v, want %#v", gotArguments, wantArguments)
	}
}

func TestResolveNupkgIDWithRunnerExplicitIDDoesNotExecute(t *testing.T) {
	runner := func(string, []string) (string, error) {
		t.Fatal("runner should not be called when package_id is explicit")
		return "", nil
	}

	id, err := resolveNupkgIDWithRunner("Chef Client", `C:\packages`, "--version=1.2.3", " chef-client ", runner)
	if err != nil {
		t.Fatalf("resolveNupkgIDWithRunner returned error: %v", err)
	}
	if id != "chef-client" {
		t.Fatalf("id = %q, want %q", id, "chef-client")
	}
}

func TestResolveNupkgIDWithRunnerPropagatesLookupError(t *testing.T) {
	lookupErr := errors.New("lookup failed")
	runner := func(string, []string) (string, error) {
		return "", lookupErr
	}

	_, err := resolveNupkgIDWithRunner("Chef Client", `C:\packages`, "--version=1.2.3", "", runner)
	if !errors.Is(err, lookupErr) {
		t.Fatalf("error = %v, want %v", err, lookupErr)
	}
}

func TestInstallItemWithRunnerBuildsMSICommandWithoutExecutingInstaller(t *testing.T) {
	originalInstalledItems := report.InstalledItems
	t.Cleanup(func() { report.InstalledItems = originalInstalledItems })

	item := catalog.Item{
		DisplayName: "MSI Test",
		Installer: catalog.InstallerItem{
			Type:      "msi",
			Location:  "packages/chef-client/chef-client-14.3.37-1-x64.msi",
			Hash:      "a1d4982abbb2bd2ccc238372ae688c790659c2c120efcee329fcca49c7c8fa9a",
			Arguments: []string{"/L=1033", "/S"},
		},
	}

	var gotCommand string
	var gotArguments []string
	runner := func(command string, arguments []string) (string, error) {
		gotCommand = command
		gotArguments = append([]string(nil), arguments...)
		return "fake installer output", nil
	}

	gotOutput := installItemWithRunner(item, "https://example.com/chef-client/chef-client-14.3.37-1-x64.msi", "testdata/", runner)
	if gotOutput != "fake installer output" {
		t.Fatalf("output = %q, want %q", gotOutput, "fake installer output")
	}
	if gotCommand != commandMsi {
		t.Fatalf("command = %q, want %q", gotCommand, commandMsi)
	}
	absFile := filepath.Join("testdata", "packages/chef-client/chef-client-14.3.37-1-x64.msi")
	wantArguments := []string{"/i", absFile, "/qn", "/norestart", "/L=1033", "/S"}
	if !reflect.DeepEqual(gotArguments, wantArguments) {
		t.Fatalf("arguments = %#v, want %#v", gotArguments, wantArguments)
	}
}

func TestUninstallItemWithRunnerBuildsMSIXCommandWithoutExecutingPowerShell(t *testing.T) {
	originalUninstalledItems := report.UninstalledItems
	t.Cleanup(func() { report.UninstalledItems = originalUninstalledItems })

	item := catalog.Item{
		DisplayName: "MSIX Test",
		Installer:   catalog.InstallerItem{Type: "msix"},
		Uninstaller: catalog.InstallerItem{Type: "msix"},
		Check: catalog.InstallCheck{
			Appx: catalog.AppxCheck{Name: "Gorilla.Test.App"},
		},
	}

	var gotCommand string
	var gotArguments []string
	runner := func(command string, arguments []string) (string, error) {
		gotCommand = command
		gotArguments = append([]string(nil), arguments...)
		return "fake uninstall output", nil
	}

	gotOutput := uninstallItemWithRunner(item, "", "testdata/", runner)
	if gotOutput != "fake uninstall output" {
		t.Fatalf("output = %q, want %q", gotOutput, "fake uninstall output")
	}
	if gotCommand != commandPs1 {
		t.Fatalf("command = %q, want %q", gotCommand, commandPs1)
	}
	if len(gotArguments) != 7 || gotArguments[5] != "-Command" {
		t.Fatalf("unexpected PowerShell arguments: %#v", gotArguments)
	}
	wantCommand := "$pkg = Get-AppxProvisionedPackage -Online | Where-Object { $_.DisplayName -eq 'Gorilla.Test.App' }; if ($pkg) { Remove-AppxProvisionedPackage -Online -PackageName $pkg.PackageName }; Get-AppxPackage -Name 'Gorilla.Test.App' -AllUsers | Remove-AppxPackage -AllUsers -ErrorAction SilentlyContinue"
	if gotArguments[6] != wantCommand {
		t.Fatalf("PowerShell command = %q, want %q", gotArguments[6], wantCommand)
	}
}
