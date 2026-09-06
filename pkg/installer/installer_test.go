package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/download"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	"github.com/1dustindavis/gorilla/pkg/report"
)

// A lot of ideas taken from https://npf.io/2015/06/testing-exec-command/

var (
	// store original data to restore after each test
	origExec            = execCommand
	origCheckStatus     = statusCheckStatus
	origReportInstalled = report.InstalledItems
	origInstallItemFunc = installItemFunc
	origRunCommand      = runCommand

	// These tore the URL that `Install` generates during testing
	installItemURL   string
	uninstallItemURL string

	// Define a testing config for `download`
	downloadCfg = config.Configuration{
		CachePath: "testdata/",
	}
	// CheckOnly flag disabled for testing
	checkOnlyMode bool = false

	// These catalog items provide test data for each installer type
	nupkgItem = catalog.Item{
		Installer: catalog.InstallerItem{
			Arguments: []string{`/L=1033`, `/S`},
			Hash:      `f441893d760c411c25420a0cb4ba3a2c708fa69d7ed455818bef1a5fd4ae7577`,
			Location:  `packages/chef-client/chef-client-14.3.37-1-x64.nupkg`,
			Type:      `nupkg`,
		},
		Uninstaller: catalog.InstallerItem{
			Arguments: []string{`/U=1033`, `/S`},
			Hash:      `a3fb64e1cadce0fd5bd08a7b01ce991c8c8bfb5618fa7e0975b6a7387dc26cba`,
			Location:  `packages/chef-client/chef-client-14.3.37-1-x64uninst.nupkg`,
			Type:      `nupkg`,
		},
		Version: "1.2.3",
	}
	msiItem = catalog.Item{
		Installer: catalog.InstallerItem{
			Arguments: []string{`/L=1033`, `/S`},
			Hash:      `a1d4982abbb2bd2ccc238372ae688c790659c2c120efcee329fcca49c7c8fa9a`,
			Location:  `packages/chef-client/chef-client-14.3.37-1-x64.msi`,
			Type:      `msi`,
		},
		Uninstaller: catalog.InstallerItem{
			Arguments: []string{`/U=1033`, `/S`},
			Hash:      `069068fea26346a7c006f39f8d84ced2ebb6b874a35143f52ed979d29f11ef3d`,
			Location:  `packages/chef-client/chef-client-14.3.37-1-x64uninst.msi`,
			Type:      `msi`,
		},
		Version: "1.2.3",
	}
	exeItem = catalog.Item{
		Installer: catalog.InstallerItem{
			Arguments: []string{`/L=1033`, `/S`},
			Hash:      `7235428c924193a353db253c59cfbf1501299df6fefcb23fa577ea96612473da`,
			Location:  `packages/chef-client/chef-client-14.3.37-1-x64.exe`,
			Type:      `exe`,
		},
		Uninstaller: catalog.InstallerItem{
			Arguments: []string{`/U=1033`, `/S`},
			Hash:      `9dc6a2c1c1ae2c3f399d7ac3c01eb5ac2976e55e8bedb842755eebe3b9add9e7`,
			Location:  `packages/chef-client/chef-client-14.3.37-1-x64uninst.exe`,
			Type:      `exe`,
		},
	}
	ps1Item = catalog.Item{
		Installer: catalog.InstallerItem{
			Arguments: []string{`/L=1033`, `/S`},
			Hash:      `195f5d4d521ca39f96b7d8fd5edd96d1f129493ddb56ae1c5c6db6cefe2167ee`,
			Location:  `packages/chef-client/chef-client-14.3.37-1-x64.ps1`,
			Type:      `ps1`,
		},
		Uninstaller: catalog.InstallerItem{
			Arguments: []string{`/U=1033`, `/S`},
			Hash:      `0c6f40ae30bcf5e3658bef5122037c927b72bc5a6e0bbf48d7294a0e453d620e`,
			Location:  `packages/chef-client/chef-client-14.3.37-1-x64uninst.ps1`,
			Type:      `ps1`,
		},
	}
	msixItem = catalog.Item{
		Installer: catalog.InstallerItem{
			Hash:     `fcc18ed417b62314901e2712933f89e55d5900f2c3cf883100e7fcef1ef1de74`,
			Location: `packages/chef-client/chef-client-14.3.37-1-x64.msix`,
			Type:     `msix`,
		},
		Uninstaller: catalog.InstallerItem{
			Type: `msix`,
		},
		Check: catalog.InstallCheck{
			Appx: catalog.AppxCheck{
				Name: `Gorilla.Test.App`,
			},
		},
		Version: "1.0.0",
	}

	// Define different options to bypass status checks during tests
	statusActionNoError   = `_gorilla_dev_action_noerror_`
	statusNoActionNoError = `_gorilla_dev_noaction_noerror_`
	statusActionError     = `_gorilla_dev_action_error_`
	statusNoActionError   = `_gorilla_dev_noaction_error_`
)

// fakeExecCommand provides a method for validating what is passed to exec.Command
// this function was copied verbatim from https://npf.io/2015/06/testing-exec-command/
func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

// fakeRunCommand just returns a string and error interface
func fakeRunCommand(command string, arguments []string) (string, error) {
	cmdOutput := "This is a fake test command return"
	var err error
	if msiItem.DisplayName == statusActionNoError {
		err = nil
	} else if msiItem.DisplayName == statusActionError {
		err = fmt.Errorf("Deliberate test error has occurred!!")
	}

	return cmdOutput, err
}

func fakeInstallerRunner(nupkgID string) commandRunner {
	return func(command string, arguments []string) (string, error) {
		if command == commandNupkg && len(arguments) > 0 && arguments[0] == "list" {
			return nupkgID, nil
		}
		return fmt.Sprint(append([]string{command}, arguments...)), nil
	}
}

// TestHelperProcess processes the commands passed to fakeExecCommand
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	// print the command we received
	fmt.Print(os.Args[3:])
	os.Exit(0)
}

func fakeCheckStatus(catalogItem catalog.Item, installType string, cachePath string) (install bool, checkErr error) {
	// Catch special names used in tests
	if catalogItem.DisplayName == statusActionNoError {
		gorillalog.Warn("Running Development Tests!")
		gorillalog.Warn(catalogItem.DisplayName)
		return true, nil
	} else if catalogItem.DisplayName == statusNoActionNoError {
		gorillalog.Warn("Running Development Tests!")
		gorillalog.Warn(catalogItem.DisplayName)
		return false, nil
	} else if catalogItem.DisplayName == statusActionError {
		gorillalog.Warn("Running Development Tests!")
		gorillalog.Warn(catalogItem.DisplayName)
		return true, fmt.Errorf("testing %v", catalogItem.DisplayName)
	} else if catalogItem.DisplayName == statusNoActionError {
		gorillalog.Warn("Running Development Tests!")
		gorillalog.Warn(catalogItem.DisplayName)
		return false, fmt.Errorf("testing %v", catalogItem.DisplayName)
	}

	fmt.Println(catalogItem.DisplayName)
	fmt.Println(installType)
	return false, nil
}

// TestRunCommand verifies that the command and it's arguments are processed correctly
func TestRunCommand(t *testing.T) {
	// Override execCommand with our fake version
	execCommand = fakeExecCommand
	defer func() { execCommand = origExec }()

	// Define our test command and arguments
	testCommand := "echo"
	testArgs := []string{"pizza", "sushi"}
	testCmd := append([]string{testCommand}, testArgs...)
	expectedCmd := fmt.Sprint(testCmd)

	actualCmd, _ := runCommand(testCommand, testArgs)

	// Compare the result with our expectations
	structsMatch := reflect.DeepEqual(expectedCmd, actualCmd)

	if !structsMatch {
		t.Errorf("\nExpected: %#v\nReceived: %#v", expectedCmd, actualCmd)
	}
}

// TestInstallItem validates the command selected for each installer type.
func TestInstallItem(t *testing.T) {
	download.SetConfig(downloadCfg)
	runner := fakeInstallerRunner("chef-client")

	// Set shared testing variables
	cachePath := "testdata/"
	pkgCache := "testdata/packages/"
	urlPackages := "https://example.com/"

	//
	// Nupkg
	//
	nupkgItem.DisplayName = statusActionNoError
	nupkgPath := "chef-client/chef-client-14.3.37-1-x64.nupkg"
	nupkgURL := urlPackages + nupkgPath

	// Run Install
	actualNupkg := installItemWithRunner(nupkgItem, nupkgURL, cachePath, runner)

	// Check the result
	nupkgCmd := filepath.Join(os.Getenv("ProgramData"), "chocolatey/bin/choco.exe")
	nupkgFile := filepath.Join(pkgCache, nupkgPath)
	nupkgDir := filepath.Dir(nupkgFile)
	expectedNupkg := fmt.Sprintf("[%s install chef-client -s %s --version=1.2.3 -f -y -r]", nupkgCmd, nupkgDir)
	if have, want := actualNupkg, expectedNupkg; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	//
	// Msi
	//
	msiItem.DisplayName = statusActionNoError
	msiPath := "chef-client/chef-client-14.3.37-1-x64.msi"
	msiURL := urlPackages + msiPath

	// Run Install
	actualMsi := installItemWithRunner(msiItem, msiURL, cachePath, runner)

	// Check the result
	msiCmd := filepath.Join(os.Getenv("WINDIR"), "system32/msiexec.exe")
	msiFile := filepath.Join(pkgCache, msiPath)
	expectedMsi := "[" + msiCmd + " /i " + msiFile + " /qn /norestart /L=1033 /S]"
	if have, want := actualMsi, expectedMsi; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	//
	// Exe
	//
	exeItem.DisplayName = statusActionNoError
	// Run Install
	actualExe := installItemWithRunner(exeItem, urlPackages, cachePath, runner)
	// Check the result
	exeFile := filepath.Clean("testdata/packages/chef-client/chef-client-14.3.37-1-x64.exe")
	expectedExe := "[" + exeFile + " /L=1033 /S]"
	if have, want := actualExe, expectedExe; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	//
	// Ps1
	//
	ps1Item.DisplayName = statusActionNoError
	// Run Install
	actualPs1 := installItemWithRunner(ps1Item, urlPackages, cachePath, runner)
	// Check the result
	ps1Cmd := filepath.Join(os.Getenv("WINDIR"), "system32/WindowsPowershell/v1.0/powershell.exe")
	ps1Path := filepath.Clean("testdata/packages/chef-client/chef-client-14.3.37-1-x64.ps1")
	expectedPs1 := "[" + ps1Cmd + " -NoProfile -NoLogo -NonInteractive -ExecutionPolicy Bypass -File " + ps1Path + "]"
	if have, want := actualPs1, expectedPs1; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	//
	// Msix
	//
	msixItem.DisplayName = statusActionNoError
	// Run Install
	actualMsix := installItemWithRunner(msixItem, urlPackages, cachePath, runner)
	// Check the result
	msixCmd := filepath.Join(os.Getenv("WINDIR"), "system32/WindowsPowershell/v1.0/powershell.exe")
	msixPath := filepath.Clean("testdata/packages/chef-client/chef-client-14.3.37-1-x64.msix")
	expectedMsix := "[" + msixCmd + " -NoProfile -NoLogo -NonInteractive -ExecutionPolicy Bypass -Command Add-AppxProvisionedPackage -Online -PackagePath '" + msixPath + "' -SkipLicense]"
	if have, want := actualMsix, expectedMsix; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

}

// TestInstallStatusError verifies that Install returns if status check fails
func TestInstallStatusError(t *testing.T) {
	statusCheckStatus = fakeCheckStatus
	defer func() { statusCheckStatus = origCheckStatus }()

	msiItem.DisplayName = statusActionError
	actualOutput := Install(msiItem, "install", "https://example.com", "testdata/", checkOnlyMode)
	expectedOutput := "Unable to check status: testing _gorilla_dev_action_error_"
	if have, want := actualOutput, expectedOutput; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}
}

// TestInstallStatusFalse verifies that Install returns if status check is false
func TestInstallStatusFalse(t *testing.T) {
	statusCheckStatus = fakeCheckStatus
	defer func() { statusCheckStatus = origCheckStatus }()

	msiItem.DisplayName = statusNoActionNoError
	actualOutput := Install(msiItem, "install", "https://example.com/", "testdata/", checkOnlyMode)
	expectedOutput := "Item not needed"
	if have, want := actualOutput, expectedOutput; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}
}

// TestUninstallItem validates the command selected for each installer type.
func TestUninstallItem(t *testing.T) {
	download.SetConfig(downloadCfg)
	runner := fakeInstallerRunner("chef-client")

	cachePath := "testdata/"
	urlPackages := "https://example.com/"

	nupkgItem.DisplayName = statusNoActionNoError
	actualNupkg := uninstallItemWithRunner(nupkgItem, urlPackages, cachePath, runner)
	nupkgCmd := filepath.Join(os.Getenv("ProgramData"), "chocolatey/bin/choco.exe")
	nupkgFile := filepath.Clean("testdata/packages/chef-client/chef-client-14.3.37-1-x64uninst.nupkg")
	nupkgDir := filepath.Dir(nupkgFile)
	expectedNupkg := fmt.Sprintf("[%s uninstall chef-client -s %s --version=1.2.3 -f -y -r]", nupkgCmd, nupkgDir)
	if have, want := actualNupkg, expectedNupkg; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	msiItem.DisplayName = statusNoActionNoError
	actualMsi := uninstallItemWithRunner(msiItem, urlPackages, cachePath, runner)
	msiCmd := filepath.Join(os.Getenv("WINDIR"), "system32/msiexec.exe")
	msiPath := filepath.Clean("testdata/packages/chef-client/chef-client-14.3.37-1-x64uninst.msi")
	expectedMsi := "[" + msiCmd + " /x " + msiPath + " /qn /norestart]"
	if have, want := actualMsi, expectedMsi; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	exeItem.DisplayName = statusNoActionNoError
	actualExe := uninstallItemWithRunner(exeItem, urlPackages, cachePath, runner)
	exePath := filepath.Clean("testdata/packages/chef-client/chef-client-14.3.37-1-x64uninst.exe")
	expectedExe := "[" + exePath + " /U=1033 /S]"
	if have, want := actualExe, expectedExe; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	ps1Item.DisplayName = statusNoActionNoError
	actualPs1 := uninstallItemWithRunner(ps1Item, urlPackages, cachePath, runner)
	ps1Cmd := filepath.Join(os.Getenv("WINDIR"), "system32/WindowsPowershell/v1.0/powershell.exe")
	ps1Path := filepath.Clean("testdata/packages/chef-client/chef-client-14.3.37-1-x64uninst.ps1")
	expectedPs1 := "[" + ps1Cmd + " -NoProfile -NoLogo -NonInteractive -ExecutionPolicy Bypass -File " + ps1Path + "]"
	if have, want := actualPs1, expectedPs1; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	msixItem.DisplayName = statusNoActionNoError
	actualMsix := uninstallItemWithRunner(msixItem, "", cachePath, runner)
	msixCmd := filepath.Join(os.Getenv("WINDIR"), "system32/WindowsPowershell/v1.0/powershell.exe")
	expectedMsix := "[" + msixCmd + " -NoProfile -NoLogo -NonInteractive -ExecutionPolicy Bypass -Command $pkg = Get-AppxProvisionedPackage -Online | Where-Object { $_.DisplayName -eq 'Gorilla.Test.App' }; if ($pkg) { Remove-AppxProvisionedPackage -Online -PackageName $pkg.PackageName }; Get-AppxPackage -Name 'Gorilla.Test.App' -AllUsers | Remove-AppxPackage -AllUsers -ErrorAction SilentlyContinue]"
	if have, want := actualMsix, expectedMsix; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}
}

func TestInstallItemNupkgWithExplicitPackageID(t *testing.T) {
	execCommand = fakeExecCommand
	runCommand = origRunCommand
	defer func() {
		execCommand = origExec
		runCommand = origRunCommand
	}()

	cachePath := "testdata/"
	pkgCache := "testdata/packages/"
	urlPackages := "https://example.com/"

	nupkgPath := "chef-client/chef-client-14.3.37-1-x64.nupkg"
	nupkgURL := urlPackages + nupkgPath
	nupkgFile := filepath.Join(pkgCache, nupkgPath)
	nupkgDir := filepath.Dir(nupkgFile)

	item := nupkgItem
	item.DisplayName = statusActionNoError
	item.Installer.PackageID = "chef-client"

	actual := installItem(item, nupkgURL, cachePath)
	expected := fmt.Sprintf("[%s install chef-client -s %s --version=1.2.3 -f -y -r]", commandNupkg, nupkgDir)
	if have, want := actual, expected; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}
}

func TestInstallItemNupkgAmbiguousPackageID(t *testing.T) {
	runCommand = func(command string, arguments []string) (string, error) {
		if len(arguments) > 0 && arguments[0] == "list" {
			return "chef-client\nchef-client-alt", nil
		}
		t.Fatalf("unexpected install command was executed: %s %s", command, strings.Join(arguments, " "))
		return "", nil
	}
	defer func() { runCommand = origRunCommand }()

	cachePath := "testdata/"
	urlPackages := "https://example.com/"
	nupkgPath := "chef-client/chef-client-14.3.37-1-x64.nupkg"
	nupkgURL := urlPackages + nupkgPath

	item := nupkgItem
	item.DisplayName = "Ambiguous Package"
	item.Installer.PackageID = ""

	actual := installItem(item, nupkgURL, cachePath)
	if !strings.Contains(actual, "Unable to determine nupkg id") || !strings.Contains(actual, "multiple package ids were found") {
		t.Fatalf("expected ambiguity error message, got: %s", actual)
	}
}

func TestUninstallItemNupkgWithExplicitPackageID(t *testing.T) {
	execCommand = fakeExecCommand
	runCommand = origRunCommand
	defer func() {
		execCommand = origExec
		runCommand = origRunCommand
	}()

	cachePath := "testdata/"
	pkgCache := "testdata/packages/"
	urlPackages := "https://example.com/"

	nupkgPath := "chef-client/chef-client-14.3.37-1-x64uninst.nupkg"
	nupkgURL := urlPackages + nupkgPath
	nupkgFile := filepath.Join(pkgCache, nupkgPath)
	nupkgDir := filepath.Dir(nupkgFile)

	item := nupkgItem
	item.DisplayName = statusNoActionNoError
	item.Uninstaller.PackageID = "chef-client"

	actual := uninstallItem(item, nupkgURL, cachePath)
	expected := fmt.Sprintf("[%s uninstall chef-client -s %s --version=1.2.3 -f -y -r]", commandNupkg, nupkgDir)
	if have, want := actual, expected; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}
}

func TestUninstallItemNupkgAmbiguousPackageID(t *testing.T) {
	runCommand = func(command string, arguments []string) (string, error) {
		if len(arguments) > 0 && arguments[0] == "list" {
			return "chef-client\nchef-client-alt", nil
		}
		t.Fatalf("unexpected uninstall command was executed: %s %s", command, strings.Join(arguments, " "))
		return "", nil
	}
	defer func() { runCommand = origRunCommand }()

	cachePath := "testdata/"
	urlPackages := "https://example.com/"
	nupkgPath := "chef-client/chef-client-14.3.37-1-x64uninst.nupkg"
	nupkgURL := urlPackages + nupkgPath

	item := nupkgItem
	item.DisplayName = "Ambiguous Uninstall Package"
	item.Uninstaller.PackageID = ""

	actual := uninstallItem(item, nupkgURL, cachePath)
	if !strings.Contains(actual, "Unable to determine nupkg id") || !strings.Contains(actual, "multiple package ids were found") {
		t.Fatalf("expected ambiguity error message, got: %s", actual)
	}
}

func TestUninstallItemMsixMissingName(t *testing.T) {
	execCommand = fakeExecCommand
	defer func() { execCommand = origExec }()

	item := msixItem
	item.DisplayName = "Missing Name App"
	item.Check.Appx.Name = ""

	actual := uninstallItem(item, "", "testdata/")
	expected := "Check.Appx.Name is required for msix uninstall of Missing Name App"
	if have, want := actual, expected; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}
}

func TestUninstallStatusError(t *testing.T) {
	statusCheckStatus = fakeCheckStatus
	defer func() { statusCheckStatus = origCheckStatus }()

	msiItem.DisplayName = statusNoActionError
	actualOutput := Install(msiItem, "uninstall", "https://example.com", "testdata/", checkOnlyMode)
	expectedOutput := "Unable to check status: testing _gorilla_dev_noaction_error_"
	if have, want := actualOutput, expectedOutput; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}
}

func TestUninstallStatusTrue(t *testing.T) {
	statusCheckStatus = fakeCheckStatus
	defer func() { statusCheckStatus = origCheckStatus }()

	msiItem.DisplayName = statusNoActionNoError
	actualOutput := Install(msiItem, "uninstall", "https://example.com", "testdata/", checkOnlyMode)
	expectedOutput := "Item not needed"
	if have, want := actualOutput, expectedOutput; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}
}

func TestUpdateStatusError(t *testing.T) {
	statusCheckStatus = fakeCheckStatus
	defer func() { statusCheckStatus = origCheckStatus }()

	msiItem.DisplayName = statusActionError
	actualOutput := Install(msiItem, "update", "https://example.com", "testdata/", checkOnlyMode)
	expectedOutput := "Unable to check status: testing _gorilla_dev_action_error_"
	if have, want := actualOutput, expectedOutput; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}
}

func TestUpdateStatusFalse(t *testing.T) {
	statusCheckStatus = fakeCheckStatus
	defer func() { statusCheckStatus = origCheckStatus }()

	msiItem.DisplayName = statusNoActionNoError
	actualOutput := Install(msiItem, "update", "https://example.com", "testdata/", checkOnlyMode)
	expectedOutput := "Item not needed"
	if have, want := actualOutput, expectedOutput; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}
}

func TestInstallReport(t *testing.T) {
	execCommand = fakeExecCommand
	report.InstalledItems = []interface{}{}
	defer func() {
		execCommand = origExec
		report.InstalledItems = origReportInstalled
	}()

	installItem(msiItem, "https://example.com", "testdata/")

	expectedReport := []interface{}{msiItem}
	structsMatch := reflect.DeepEqual(expectedReport, report.InstalledItems)
	if !structsMatch {
		t.Errorf("\nExpected: %#v\nReceived: %#v", expectedReport, report.InstalledItems)
	}
}

func fakeInstallItem(item catalog.Item, itemURL, cachePath string) (string, error) {
	installItemURL = itemURL
	return "", nil
}

func TestInstallURL(t *testing.T) {
	statusCheckStatus = fakeCheckStatus
	installItemFunc = fakeInstallItem
	defer func() {
		statusCheckStatus = origCheckStatus
		installItemFunc = origInstallItemFunc
	}()

	installItemURL = ""
	msiItem.DisplayName = statusActionNoError
	Install(msiItem, "install", "https://example.com/", "testdata/", checkOnlyMode)
	expectedURL := "https://example.com/packages/chef-client/chef-client-14.3.37-1-x64.msi"
	if have, want := installItemURL, expectedURL; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}
}

func fakeUninstallItem(item catalog.Item, itemURL, cachePath string) (string, error) {
	uninstallItemURL = itemURL
	return "", nil
}

func TestUninstallURL(t *testing.T) {
	statusCheckStatus = fakeCheckStatus
	uninstallItemFunc = fakeUninstallItem
	defer func() {
		statusCheckStatus = origCheckStatus
		installItemFunc = origInstallItemFunc
	}()

	uninstallItemURL = ""
	msiItem.DisplayName = statusActionNoError
	Install(msiItem, "uninstall", "https://example.com/", "testdata/", checkOnlyMode)
	expectedURL := "https://example.com/packages/chef-client/chef-client-14.3.37-1-x64.msi"
	if have, want := uninstallItemURL, expectedURL; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}
}

func Example_runCommand() {
	logTmp, _ := os.MkdirTemp("", "gorilla-installer_test")
	cfgVerbose := config.Configuration{
		Debug:       true,
		Verbose:     true,
		AppDataPath: logTmp,
	}
	_ = gorillalog.NewLog(cfgVerbose)

	execCommand = fakeExecCommand
	defer func() { execCommand = origExec }()

	testCmd := "Command Test!"
	testArgs := []string{"arg1", "arg2"}
	runCommand(testCmd, testArgs)

	// Output:
	// command: Command Test! [arg1 arg2]
	// Command Output:
	// --------------------
	// [Command Test! arg1 arg2]
	// --------------------
}

func Example_installItemSuccess() {
	execCommand = fakeExecCommand
	statusCheckStatus = fakeCheckStatus
	runCommand = fakeRunCommand
	download.SetConfig(downloadCfg)
	defer func() {
		execCommand = origExec
		statusCheckStatus = origCheckStatus
		runCommand = origRunCommand
	}()

	cachePath := "testdata/"
	urlPackages := "https://example.com/"
	msiItem.DisplayName = statusActionNoError
	installItem(msiItem, urlPackages, cachePath)

	// Output:
	// Installing msi for _gorilla_dev_action_noerror_
	// _gorilla_dev_action_noerror_ 1.2.3 Installation SUCCESSFUL
}

func Example_installItemFailure() {
	execCommand = fakeExecCommand
	statusCheckStatus = fakeCheckStatus
	runCommand = fakeRunCommand
	download.SetConfig(downloadCfg)
	defer func() {
		execCommand = origExec
		statusCheckStatus = origCheckStatus
		runCommand = origRunCommand
	}()

	cachePath := "testdata/"
	urlPackages := "https://example.com/"
	msiItem.DisplayName = statusActionError
	installItem(msiItem, urlPackages, cachePath)

	// Output:
	// Installing msi for _gorilla_dev_action_error_
	// _gorilla_dev_action_error_ 1.2.3 Installation FAILED
}

func Example_uninstallItemSuccess() {
	execCommand = fakeExecCommand
	statusCheckStatus = fakeCheckStatus
	runCommand = fakeRunCommand
	download.SetConfig(downloadCfg)
	defer func() {
		execCommand = origExec
		statusCheckStatus = origCheckStatus
		runCommand = origRunCommand
	}()

	cachePath := "testdata/"
	urlPackages := "https://example.com/"
	msiItem.DisplayName = statusActionNoError
	uninstallItem(msiItem, urlPackages, cachePath)

	// Output:
	// Uninstalling msi for _gorilla_dev_action_noerror_
	// _gorilla_dev_action_noerror_ 1.2.3 Uninstallation SUCCESSFUL
}

func Example_uninstallItemFailure() {
	execCommand = fakeExecCommand
	statusCheckStatus = fakeCheckStatus
	runCommand = fakeRunCommand
	download.SetConfig(downloadCfg)
	defer func() {
		execCommand = origExec
		statusCheckStatus = origCheckStatus
		runCommand = origRunCommand
	}()

	cachePath := "testdata/"
	urlPackages := "https://example.com/"
	msiItem.DisplayName = statusActionError
	uninstallItem(msiItem, urlPackages, cachePath)

	// Output:
	// Uninstalling msi for _gorilla_dev_action_error_
	// _gorilla_dev_action_error_ 1.2.3 Uninstallation FAILED
}
