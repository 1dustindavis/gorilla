package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	"github.com/1dustindavis/gorilla/pkg/report"
)

// A lot of ideas taken from https://npf.io/2015/06/testing-exec-command/

var (
	// store original data to restore after each test
	origExec            = execCommand
	origCheckStatus     = statusCheckStatus
	origCachePath       = config.CachePath
	origURLPackages     = config.Current.URLPackages
	origURL             = config.Current.URL
	origReportInstalled = report.InstalledItems

	// These catalog items provide test data for each installer type
	nupkgItem = catalog.Item{
		InstallerItemArguments:   []string{`/L=1033`, `/S`},
		InstallerItemHash:        `f441893d760c411c25420a0cb4ba3a2c708fa69d7ed455818bef1a5fd4ae7577`,
		InstallerItemLocation:    `packages/chef-client/chef-client-14.3.37-1-x64.nupkg`,
		InstallerType:            `nupkg`,
		UninstallerItemArguments: []string{`/U=1033`, `/S`},
		UninstallerItemHash:      `a3fb64e1cadce0fd5bd08a7b01ce991c8c8bfb5618fa7e0975b6a7387dc26cba`,
		UninstallerItemLocation:  `packages/chef-client/chef-client-14.3.37-1-x64uninst.nupkg`,
		UninstallerType:          `nupkg`,
	}
	msiItem = catalog.Item{
		InstallerItemArguments:   []string{`/L=1033`, `/S`},
		InstallerItemHash:        `a1d4982abbb2bd2ccc238372ae688c790659c2c120efcee329fcca49c7c8fa9a`,
		InstallerItemLocation:    `packages/chef-client/chef-client-14.3.37-1-x64.msi`,
		InstallerType:            `msi`,
		UninstallerItemArguments: []string{`/U=1033`, `/S`},
		UninstallerItemHash:      `069068fea26346a7c006f39f8d84ced2ebb6b874a35143f52ed979d29f11ef3d`,
		UninstallerItemLocation:  `packages/chef-client/chef-client-14.3.37-1-x64uninst.msi`,
		UninstallerType:          `msi`,
	}
	exeItem = catalog.Item{
		InstallerItemArguments:   []string{`/L=1033`, `/S`},
		InstallerItemHash:        `7235428c924193a353db253c59cfbf1501299df6fefcb23fa577ea96612473da`,
		InstallerItemLocation:    `packages/chef-client/chef-client-14.3.37-1-x64.exe`,
		InstallerType:            `exe`,
		UninstallerItemArguments: []string{`/U=1033`, `/S`},
		UninstallerItemHash:      `9dc6a2c1c1ae2c3f399d7ac3c01eb5ac2976e55e8bedb842755eebe3b9add9e7`,
		UninstallerItemLocation:  `packages/chef-client/chef-client-14.3.37-1-x64uninst.exe`,
		UninstallerType:          `exe`,
	}
	ps1Item = catalog.Item{
		InstallerItemArguments:   []string{`/L=1033`, `/S`},
		InstallerItemHash:        `195f5d4d521ca39f96b7d8fd5edd96d1f129493ddb56ae1c5c6db6cefe2167ee`,
		InstallerItemLocation:    `packages/chef-client/chef-client-14.3.37-1-x64.ps1`,
		InstallerType:            `ps1`,
		UninstallerItemArguments: []string{`/U=1033`, `/S`},
		UninstallerItemHash:      `0c6f40ae30bcf5e3658bef5122037c927b72bc5a6e0bbf48d7294a0e453d620e`,
		UninstallerItemLocation:  `packages/chef-client/chef-client-14.3.37-1-x64uninst.ps1`,
		UninstallerType:          `ps1`,
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

func fakeCheckStatus(catalogItem catalog.Item, installType string) (install bool, checkErr error) {

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
	// Override the cachepath to use our test directory
	config.CachePath = "testdata/"
	defer func() {
		execCommand = origExec
		statusCheckStatus = origCheckStatus
		config.CachePath = origCachePath
	}()

	//
	// Nupkg
	//
	nupkgItem.DisplayName = statusActionNoError
	// Run Install
	actualNupkg := installItem(nupkgItem)
	// Check the result
	nupkgCmd := filepath.Join(os.Getenv("ProgramData"), "chocolatey/bin/choco.exe")
	nupkgPath := filepath.Clean("testdata/packages/chef-client/chef-client-14.3.37-1-x64.nupkg")
	expectedNupkg := "[" + nupkgCmd + " install " + nupkgPath + " -f -y -r]"
	if have, want := actualNupkg, expectedNupkg; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	//
	// Msi
	//
	msiItem.DisplayName = statusActionNoError
	// Run Install
	actualMsi := installItem(msiItem)
	// Check the result
	msiCmd := filepath.Join(os.Getenv("WINDIR"), "system32/msiexec.exe")
	msiPath := filepath.Clean("testdata/packages/chef-client/chef-client-14.3.37-1-x64.msi")
	expectedMsi := "[" + msiCmd + " /i " + msiPath + " /qn /norestart]"
	if have, want := actualMsi, expectedMsi; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	//
	// Exe
	//
	exeItem.DisplayName = statusActionNoError
	// Run Install
	actualExe := installItem(exeItem)
	// Check the result
	exePath := filepath.Clean("testdata/packages/chef-client/chef-client-14.3.37-1-x64.exe")
	expectedExe := "[" + exePath + " /L=1033 /S]"
	if have, want := actualExe, expectedExe; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	//
	// Ps1
	//
	ps1Item.DisplayName = statusActionNoError
	// Run Install
	actualPs1 := installItem(ps1Item)
	// Check the result
	ps1Cmd := filepath.Join(os.Getenv("WINDIR"), "system32/WindowsPowershell/v1.0/powershell.exe")
	ps1Path := filepath.Clean("testdata/packages/chef-client/chef-client-14.3.37-1-x64.ps1")
	expectedPs1 := "[" + ps1Cmd + " -NoProfile -NoLogo -NonInteractive -WindowStyle Normal -ExecutionPolicy Bypass -File " + ps1Path + "]"
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
	actualOutput := Install(msiItem, "install")
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
	actualOutput := Install(msiItem, "install")
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
	// Override the cachepath to use our test directory
	config.CachePath = "testdata/"
	defer func() {
		execCommand = origExec
		statusCheckStatus = origCheckStatus
		config.CachePath = origCachePath
	}()

	//
	// Nupkg
	//
	nupkgItem.DisplayName = statusNoActionNoError
	// Run Uninstall
	actualNupkg := uninstallItem(nupkgItem)
	// Check the result
	nupkgCmd := filepath.Join(os.Getenv("ProgramData"), "chocolatey/bin/choco.exe")
	nupkgPath := filepath.Clean("testdata/packages/chef-client/chef-client-14.3.37-1-x64uninst.nupkg")
	expectedNupkg := "[" + nupkgCmd + " uninstall " + nupkgPath + " -f -y -r]"
	if have, want := actualNupkg, expectedNupkg; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	//
	// Msi
	//
	msiItem.DisplayName = statusNoActionNoError
	// Run Uninstall
	actualMsi := uninstallItem(msiItem)
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
	actualExe := uninstallItem(exeItem)
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
	actualPs1 := uninstallItem(ps1Item)
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
	actualOutput := Install(msiItem, "uninstall")
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
	actualOutput := Install(msiItem, "uninstall")
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
	actualOutput := Install(msiItem, "update")
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
	actualOutput := Install(msiItem, "update")
	// Check the result
	expectedOutput := "Item not needed"
	if have, want := actualOutput, expectedOutput; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

}

// TestInstallURL verifies the URL that is generated when URLPackages is not defined
func TestInstallURL(t *testing.T) {
	// Override execCommand with our fake version
	execCommand = fakeExecCommand
	// Override the URLPackages to use a test url
	config.Current.URLPackages = ""
	// Override the URL to use a test url
	config.Current.URL = "https://example.com/gorilla/"
	// Override the cachepath to use our test directory
	config.CachePath = "testdata/"
	defer func() {
		config.Current.URLPackages = origURLPackages
		execCommand = origExec
		config.CachePath = origCachePath
		config.Current.URL = origURL
	}()

	// Run the installer
	installItem(msiItem)
	// Check the result
	expectedOutput := "https://example.com/gorilla/packages/chef-client/chef-client-14.3.37-1-x64.msi"
	if have, want := installerURL, expectedOutput; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

}

// TestInstallURLPackages verifies the URL that is generated when URLPackages is defined
func TestInstallURLPackages(t *testing.T) {
	// Override execCommand with our fake version
	execCommand = fakeExecCommand
	// Override the URLPackages to use a test url
	config.Current.URLPackages = "https://example.com/pkgurl/"
	// Override the URL to use a test url
	config.Current.URL = "https://example.com/gorilla/"
	// Override the cachepath to use our test directory
	config.CachePath = "testdata/"
	defer func() {
		config.Current.URLPackages = origURLPackages
		execCommand = origExec
		config.CachePath = origCachePath
		config.Current.URL = origURL
	}()

	// Run the installer
	installItem(msiItem)
	// Check the result
	expectedOutput := "https://example.com/pkgurl/packages/chef-client/chef-client-14.3.37-1-x64.msi"
	if have, want := installerURL, expectedOutput; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

}

// TestInstallReport verifies that an installed item is added to the report
func TestInstallReport(t *testing.T) {
	// Override execCommand with our fake version
	execCommand = fakeExecCommand
	// Override the cachepath to use our test directory
	config.CachePath = "testdata/"
	// Override the report.InstalledItems to be empty
	report.InstalledItems = []interface{}{}
	defer func() {
		execCommand = origExec
		config.CachePath = origCachePath
		report.InstalledItems = origReportInstalled
	}()

	// Run the installer
	installItem(msiItem)

	// Check the result
	expectedReport := []interface{}{msiItem}

	// Compare the result with our expectations
	structsMatch := reflect.DeepEqual(expectedReport, report.InstalledItems)

	if !structsMatch {
		t.Errorf("\nExpected: %#v\nReceived: %#v", expectedReport, report.InstalledItems)
	}

}
