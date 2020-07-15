package installer

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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
	testArgs := []string{"pizza", "pizza"}
	testCmd := append([]string{testCommand}, testArgs...)
	expectedCmd := fmt.Sprint(testCmd)

	actualCmd := runCommand(testCommand, testArgs)

	// Compare the result with our expectations
	structsMatch := reflect.DeepEqual(expectedCmd, actualCmd)

	if !structsMatch {
		t.Errorf("\nExpected: %#v\nReceived: %#v", expectedCmd, actualCmd)
	}
}

// TestInstallItem validate the command that is passed to
// exec.Command for each installer type
func TestInstallItem(t *testing.T) {
	// Override execCommand and checkStatus with our fake versions
	execCommand = fakeExecCommand
	statusCheckStatus = fakeCheckStatus
	defer func() {
		execCommand = origExec
		statusCheckStatus = origCheckStatus
	}()

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
	actualNupkg := installItem(nupkgItem, nupkgURL, cachePath)

	// Check the result
	nupkgCmd := filepath.Join(os.Getenv("ProgramData"), "chocolatey/bin/choco.exe")
	nupkgFile := filepath.Join(pkgCache, nupkgPath)
	nupkgDir := filepath.Dir(nupkgFile)
	nupkgID := fmt.Sprintf("[%s list --version=1.2.3 --id-only -r -s %s]", nupkgCmd, nupkgDir)
	expectedNupkg := fmt.Sprintf("[%s install %s -s %s --version=1.2.3 -f -y -r]", nupkgCmd, nupkgID, nupkgDir)
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
	actualMsi := installItem(msiItem, msiURL, cachePath)

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
	exePath := "chef-client/chef-client-14.3.37-1-x64.exe"
	exeURL := urlPackages + exePath

	// Run Install
	actualExe := installItem(exeItem, exeURL, cachePath)

	// Check the result
	exeFile := filepath.Join(pkgCache, exePath)
	expectedExe := "[" + exeFile + " /L=1033 /S]"
	if have, want := actualExe, expectedExe; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	//
	// Ps1
	//
	ps1Item.DisplayName = statusActionNoError
	ps1Path := "chef-client/chef-client-14.3.37-1-x64.ps1"
	ps1URL := urlPackages + ps1Path

	// Run Install
	actualPs1 := installItem(ps1Item, ps1URL, cachePath)

	// Check the result
	ps1Cmd := filepath.Join(os.Getenv("WINDIR"), "system32/WindowsPowershell/v1.0/powershell.exe")
	ps1File := filepath.Join(pkgCache, ps1Path)
	expectedPs1 := "[" + ps1Cmd + " -NoProfile -NoLogo -NonInteractive -WindowStyle Normal -ExecutionPolicy Bypass -File " + ps1File + "]"
	if have, want := actualPs1, expectedPs1; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

}

// TestInstallStatusError verifies that Install returns if status check fails
func TestInstallStatusError(t *testing.T) {
	// Override checkStatus with our fake version
	statusCheckStatus = fakeCheckStatus
	defer func() {
		statusCheckStatus = origCheckStatus
	}()

	// Run the msi installer with this status bypass to trigger an error
	msiItem.DisplayName = statusActionError
	// Run Install
	actualOutput := Install(msiItem, "install", "https://example.com", "testdata/", checkOnlyMode)
	// Check the result
	expectedOutput := "Unable to check status: testing _gorilla_dev_action_error_"
	if have, want := actualOutput, expectedOutput; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

}

// TestInstallStatusFalse verifies that Install returns if status check is false
func TestInstallStatusFalse(t *testing.T) {
	// Override checkStatus with our fake version
	statusCheckStatus = fakeCheckStatus
	defer func() {
		statusCheckStatus = origCheckStatus
	}()

	// Run the msi installer with this status bypass to make status return false
	msiItem.DisplayName = statusNoActionNoError
	// Run Install
	actualOutput := Install(msiItem, "install", "https://example.com/", "testdata/", checkOnlyMode)
	// Check the result
	expectedOutput := "Item not needed"
	if have, want := actualOutput, expectedOutput; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

}

// TestUninstallItem validate the command that is passed to
// exec.Command for each installer type
func TestUninstallItem(t *testing.T) {
	// Override execCommand and checkStatus with our fake versions
	execCommand = fakeExecCommand
	statusCheckStatus = fakeCheckStatus
	download.SetConfig(downloadCfg)
	defer func() {
		execCommand = origExec
		statusCheckStatus = origCheckStatus
	}()

	// Set shared testing variables
	cachePath := "testdata/"
	pkgCache := "testdata/packages/"
	urlPackages := "https://example.com/"

	//
	// Nupkg
	//
	nupkgItem.DisplayName = statusNoActionNoError
	nupkgPath := "chef-client/chef-client-14.3.37-1-x64uninst.nupkg"
	nupkgURL := urlPackages + nupkgPath
	// Run Uninstall
	actualNupkg := uninstallItem(nupkgItem, nupkgURL, cachePath)
	// Check the result
	nupkgCmd := filepath.Join(os.Getenv("ProgramData"), "chocolatey/bin/choco.exe")
	nupkgFile := filepath.Join(pkgCache, nupkgPath)
	nupkgDir := filepath.Dir(nupkgFile)
	nupkgID := fmt.Sprintf("[%s list --version=1.2.3 --id-only -r -s %s]", nupkgCmd, nupkgDir)
	expectedNupkg := fmt.Sprintf("[%s uninstall %s -s %s --version=1.2.3 -f -y -r]", nupkgCmd, nupkgID, nupkgDir)
	if have, want := actualNupkg, expectedNupkg; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	//
	// Msi
	//
	msiItem.DisplayName = statusNoActionNoError
	// Run Uninstall
	actualMsi := uninstallItem(msiItem, urlPackages, cachePath)
	// Check the result
	msiCmd := filepath.Join(os.Getenv("WINDIR"), "system32/msiexec.exe")
	msiPath := filepath.Clean("testdata/packages/chef-client/chef-client-14.3.37-1-x64uninst.msi")
	expectedMsi := "[" + msiCmd + " /x " + msiPath + " /qn /norestart]"
	if have, want := actualMsi, expectedMsi; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	//
	// Exe
	//
	exeItem.DisplayName = statusNoActionNoError
	// Run Uninstall
	actualExe := uninstallItem(exeItem, urlPackages, cachePath)
	// Check the result
	exePath := filepath.Clean("testdata/packages/chef-client/chef-client-14.3.37-1-x64uninst.exe")
	expectedExe := "[" + exePath + " /U=1033 /S]"
	if have, want := actualExe, expectedExe; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	//
	// Ps1
	//
	ps1Item.DisplayName = statusNoActionNoError
	// Run Uninstall
	actualPs1 := uninstallItem(ps1Item, urlPackages, cachePath)
	// Check the result
	ps1Cmd := filepath.Join(os.Getenv("WINDIR"), "system32/WindowsPowershell/v1.0/powershell.exe")
	ps1Path := filepath.Clean("testdata/packages/chef-client/chef-client-14.3.37-1-x64uninst.ps1")
	expectedPs1 := "[" + ps1Cmd + " -NoProfile -NoLogo -NonInteractive -WindowStyle Normal -ExecutionPolicy Bypass -File " + ps1Path + "]"
	if have, want := actualPs1, expectedPs1; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

}

// TestUninstallStatusError verifies that Uninstall returns if status check fails
func TestUninstallStatusError(t *testing.T) {
	// Override checkStatus with our fake version
	statusCheckStatus = fakeCheckStatus
	defer func() {
		statusCheckStatus = origCheckStatus
	}()

	// Run the msi uninstaller with this status bypass to trigger an error
	msiItem.DisplayName = statusNoActionError
	// Run Uninstall
	actualOutput := Install(msiItem, "uninstall", "https://example.com", "testdata/", checkOnlyMode)
	// Check the result
	expectedOutput := "Unable to check status: testing _gorilla_dev_noaction_error_"
	if have, want := actualOutput, expectedOutput; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

}

// TestUninstallStatusTrue verifies that Uninstall returns if status check is true
func TestUninstallStatusTrue(t *testing.T) {
	// Override checkStatus with our fake version
	statusCheckStatus = fakeCheckStatus
	defer func() {
		statusCheckStatus = origCheckStatus
	}()

	// Run the msi uninstaller with this status bypass to make status return true
	msiItem.DisplayName = statusNoActionNoError
	// Run Uninstall
	actualOutput := Install(msiItem, "uninstall", "https://example.com", "testdata/", checkOnlyMode)
	// Check the result
	expectedOutput := "Item not needed"
	if have, want := actualOutput, expectedOutput; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

}

// TestUpdateStatusError verifies that Update returns if status check fails
func TestUpdateStatusError(t *testing.T) {
	// Override checkStatus with our fake version
	statusCheckStatus = fakeCheckStatus
	defer func() {
		statusCheckStatus = origCheckStatus
	}()

	// Run the msi installer with this status bypass to trigger an error
	msiItem.DisplayName = statusActionError
	// Run Update
	actualOutput := Install(msiItem, "update", "https://example.com", "testdata/", checkOnlyMode)
	// Check the result
	expectedOutput := "Unable to check status: testing _gorilla_dev_action_error_"
	if have, want := actualOutput, expectedOutput; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

}

// TestUpdateStatusFalse verifies that Update returns if status check is false
func TestUpdateStatusFalse(t *testing.T) {
	// Override checkStatus with our fake version
	statusCheckStatus = fakeCheckStatus
	defer func() {
		statusCheckStatus = origCheckStatus
	}()

	// Run the msi installer with this status bypass to make status return dalse
	msiItem.DisplayName = statusNoActionNoError
	// Run Update
	actualOutput := Install(msiItem, "update", "https://example.com", "testdata/", checkOnlyMode)
	// Check the result
	expectedOutput := "Item not needed"
	if have, want := actualOutput, expectedOutput; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

}

// TestInstallReport verifies that an installed item is added to the report
func TestInstallReport(t *testing.T) {
	// Override execCommand with our fake version
	execCommand = fakeExecCommand
	// Override the report.InstalledItems to be empty
	report.InstalledItems = []interface{}{}
	defer func() {
		execCommand = origExec
		report.InstalledItems = origReportInstalled
	}()

	// Run the installer
	installItem(msiItem, "https://example.com", "testdata/")

	// Check the result
	expectedReport := []interface{}{msiItem}

	// Compare the result with our expectations
	structsMatch := reflect.DeepEqual(expectedReport, report.InstalledItems)

	if !structsMatch {
		t.Errorf("\nExpected: %#v\nReceived: %#v", expectedReport, report.InstalledItems)
	}

}

func fakeInstallItem(item catalog.Item, itemURL, cachePath string) string {
	installItemURL = itemURL
	return ""
}

// TestInstallURL validates that the url for an installer is properly generated
func TestInstallURL(t *testing.T) {
	// Override checkStatus and installItemFunc with our fake versions
	statusCheckStatus = fakeCheckStatus
	installItemFunc = fakeInstallItem
	defer func() {
		statusCheckStatus = origCheckStatus
		installItemFunc = origInstallItemFunc
	}()

	// Make sure the `installItemURL` variable is blank before we start
	installItemURL = ""

	// Run the msi installer with this status bypass checks
	msiItem.DisplayName = statusActionNoError

	// Run Install
	Install(msiItem, "install", "https://example.com/", "testdata/", checkOnlyMode)

	// Check the result
	expectedURL := "https://example.com/packages/chef-client/chef-client-14.3.37-1-x64.msi"

	if have, want := installItemURL, expectedURL; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}
}

func fakeUninstallItem(item catalog.Item, itemURL, cachePath string) string {
	uninstallItemURL = itemURL
	return ""
}

// TestUninstallURL validates that the url for an installer is properly generated
func TestUninstallURL(t *testing.T) {
	// Override checkStatus and installItemFunc with our fake versions
	statusCheckStatus = fakeCheckStatus
	uninstallItemFunc = fakeUninstallItem
	defer func() {
		statusCheckStatus = origCheckStatus
		installItemFunc = origInstallItemFunc
	}()

	// Make sure the `installItemURL` variable is blank before we start
	uninstallItemURL = ""

	// Run the msi installer with this status bypass checks
	msiItem.DisplayName = statusActionNoError

	// Run Install
	Install(msiItem, "uninstall", "https://example.com/", "testdata/", checkOnlyMode)

	// Check the result
	expectedURL := "https://example.com/packages/chef-client/chef-client-14.3.37-1-x64.msi"

	if have, want := installItemURL, expectedURL; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}
}

// Example_runCommand tests the output when running a command in debug
func Example_runCommand() {
	// Temp directory for logging
	logTmp, _ := ioutil.TempDir("", "gorilla-installer_test")

	// Setup a testing Configuration struct with debug mode
	cfgVerbose := config.Configuration{
		Debug:       true,
		Verbose:     true,
		AppDataPath: logTmp,
	}

	// Start gorillalog in debug mode
	gorillalog.NewLog(cfgVerbose)

	// Override execCommand with our fake version
	execCommand = fakeExecCommand
	defer func() { execCommand = origExec }()

	// Set up what we expect
	testCmd := "Command Test!"
	testArgs := []string{"arg1", "arg2"}

	// Run the function
	runCommand(testCmd, testArgs)

	// Output:
	// command: Command Test! [arg1 arg2]
	// Command Output:
	// --------------------
	// [Command Test! arg1 arg2]
	// --------------------
}
