package installer

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
)

// A lot of ideas taken from https://npf.io/2015/06/testing-exec-command/

var (
	// store original data to restore after each test
	origExec      = execCommand
	origCachePath = config.CachePath

	// These catalog items provide test data for each installer type
	nupkgItem = catalog.Item{
		InstallerItemArguments: []string{`/L=1033`, `/S`},
		InstallerItemHash:      `f441893d760c411c25420a0cb4ba3a2c708fa69d7ed455818bef1a5fd4ae7577`,
		InstallerItemLocation:  `packages/chef-client/chef-client-14.3.37-1-x64.nupkg`,
		UninstallMethod:        `choco`,
	}
	msiItem = catalog.Item{
		InstallerItemArguments: []string{`/L=1033`, `/S`},
		InstallerItemHash:      `a1d4982abbb2bd2ccc238372ae688c790659c2c120efcee329fcca49c7c8fa9a`,
		InstallerItemLocation:  `packages/chef-client/chef-client-14.3.37-1-x64.msi`,
		UninstallMethod:        `msi`,
	}
	exeItem = catalog.Item{
		InstallerItemArguments: []string{`/L=1033`, `/S`},
		InstallerItemHash:      `7235428c924193a353db253c59cfbf1501299df6fefcb23fa577ea96612473da`,
		InstallerItemLocation:  `packages/chef-client/chef-client-14.3.37-1-x64.exe`,
		UninstallMethod:        `exe`,
	}
	ps1Item = catalog.Item{
		InstallerItemArguments: []string{`/L=1033`, `/S`},
		InstallerItemHash:      `195f5d4d521ca39f96b7d8fd5edd96d1f129493ddb56ae1c5c6db6cefe2167ee`,
		InstallerItemLocation:  `packages/chef-client/chef-client-14.3.37-1-x64.ps1`,
		UninstallMethod:        `ps1`,
	}

	// Define different DisplayName options to bypass status checks
	statusInstallNoError   = `_gorilla_dev_install_noerror_`
	statusInstallError     = `_gorilla_dev_install_error_`
	statusNoInstallNoError = `_gorilla_dev_noinstall_noerror_`
	statusNoInstallError   = `_gorilla_dev_noinstall_error_`
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

// TestInstall validate the command that is passed to
// exec.Command for each installer type
func TestInstall(t *testing.T) {
	// Override execCommand with our fake version
	execCommand = fakeExecCommand
	// Override the cachepath to use our test directory
	config.CachePath = "testdata/"
	defer func() {
		execCommand = origExec
		config.CachePath = origCachePath
	}()

	//
	// Nupkg
	//
	nupkgItem.DisplayName = statusInstallNoError
	// Run Install
	actualNupkg := Install(nupkgItem)
	// Check the result
	expectedNupkg := "[chocolatey/bin/choco.exe install testdata/packages/chef-client/chef-client-14.3.37-1-x64.nupkg -y -r]"
	if have, want := actualNupkg, expectedNupkg; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	//
	// Msi
	//
	msiItem.DisplayName = statusInstallNoError
	// Run Install
	actualMsi := Install(msiItem)
	// Check the result
	expectedMsi := "[system32/msiexec.exe /i testdata/packages/chef-client/chef-client-14.3.37-1-x64.msi /qn /norestart]"
	if have, want := actualMsi, expectedMsi; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	//
	// Exe
	//
	exeItem.DisplayName = statusInstallNoError
	// Run Install
	actualExe := Install(exeItem)
	// Check the result
	expectedExe := "[testdata/packages/chef-client/chef-client-14.3.37-1-x64.exe /L=1033 /S]"
	if have, want := actualExe, expectedExe; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	//
	// Ps1
	//
	ps1Item.DisplayName = statusInstallNoError
	// Run Install
	actualPs1 := Install(ps1Item)
	// Check the result
	expectedPs1 := "[system32/WindowsPowershell/v1.0/powershell.exe -NoProfile -NoLogo -NonInteractive -WindowStyle Normal -ExecutionPolicy Bypass -File testdata/packages/chef-client/chef-client-14.3.37-1-x64.ps1]"
	if have, want := actualPs1, expectedPs1; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

}

// TestInstallStatusError verifies that Install returns if status check fails
func TestInstallStatusError(t *testing.T) {

	// Run the msi installer with this status bypass to trigger an error
	msiItem.DisplayName = statusInstallError
	// Run Install
	actualOutput := Install(msiItem)
	// Check the result
	expectedOutput := "Unable to check status: testing _gorilla_dev_install_error_"
	if have, want := actualOutput, expectedOutput; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

}

// TestInstallStatusFalse verifies that Install returns if status check is false
func TestInstallStatusFalse(t *testing.T) {

	// Run the msi installer with this status bypass to make status return false
	msiItem.DisplayName = statusNoInstallNoError
	// Run Install
	actualOutput := Install(msiItem)
	// Check the result
	expectedOutput := "Install not needed"
	if have, want := actualOutput, expectedOutput; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

}

// TestUninstall validate the command that is passed to
// exec.Command for each installer type
func TestUninstall(t *testing.T) {
	// Override execCommand with our fake version
	execCommand = fakeExecCommand
	// Override the cachepath to use our test directory
	config.CachePath = "testdata/"
	defer func() {
		execCommand = origExec
		config.CachePath = origCachePath
	}()

	//
	// Nupkg
	//
	nupkgItem.DisplayName = statusNoInstallNoError
	// Run Uninstall
	actualNupkg := Uninstall(nupkgItem)
	// Check the result
	expectedNupkg := "[chocolatey/bin/choco.exe uninstall testdata/packages/chef-client/chef-client-14.3.37-1-x64.nupkg -y -r]"
	if have, want := actualNupkg, expectedNupkg; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	//
	// Msi
	//
	msiItem.DisplayName = statusNoInstallNoError
	// Run Uninstall
	actualMsi := Uninstall(msiItem)
	// Check the result
	expectedMsi := "[system32/msiexec.exe /x testdata/packages/chef-client/chef-client-14.3.37-1-x64.msi /qn /norestart]"
	if have, want := actualMsi, expectedMsi; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	//
	// Exe
	//
	exeItem.DisplayName = statusNoInstallNoError
	// Run Uninstall
	actualExe := Uninstall(exeItem)
	// Check the result
	expectedExe := "unsupported uninstaller"
	if have, want := actualExe, expectedExe; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	//
	// Ps1
	//
	ps1Item.DisplayName = statusNoInstallNoError
	// Run Uninstall
	actualPs1 := Uninstall(ps1Item)
	// Check the result
	expectedPs1 := "unsupported uninstaller"
	if have, want := actualPs1, expectedPs1; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

}

// TestUninstallStatusError verifies that Uninstall returns if status check fails
func TestUninstallStatusError(t *testing.T) {

	// Run the msi uninstaller with this status bypass to trigger an error
	msiItem.DisplayName = statusNoInstallError
	// Run Uninstall
	actualOutput := Uninstall(msiItem)
	// Check the result
	expectedOutput := "Unable to check status: testing _gorilla_dev_noinstall_error_"
	if have, want := actualOutput, expectedOutput; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

}

// TestUninstallStatusTrue verifies that Uninstall returns if status check is true
func TestUninstallStatusTrue(t *testing.T) {

	// Run the msi uninstaller with this status bypass to make status return true
	msiItem.DisplayName = statusInstallNoError
	// Run Uninstall
	actualOutput := Uninstall(msiItem)
	// Check the result
	expectedOutput := "Uninstall not needed"
	if have, want := actualOutput, expectedOutput; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

}

// TestUpdate validate the command that is passed to
// exec.Command for each installer type
func TestUpdate(t *testing.T) {
	// Override execCommand with our fake version
	execCommand = fakeExecCommand
	// Override the cachepath to use our test directory
	config.CachePath = "testdata/"
	defer func() {
		execCommand = origExec
		config.CachePath = origCachePath
	}()

	//
	// Nupkg
	//
	nupkgItem.DisplayName = statusInstallNoError
	// Run Update
	actualNupkg := Update(nupkgItem)
	// Check the result
	expectedNupkg := "[chocolatey/bin/choco.exe install testdata/packages/chef-client/chef-client-14.3.37-1-x64.nupkg -y -r]"
	if have, want := actualNupkg, expectedNupkg; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	//
	// Msi
	//
	msiItem.DisplayName = statusInstallNoError
	// Run Update
	actualMsi := Update(msiItem)
	// Check the result
	expectedMsi := "[system32/msiexec.exe /i testdata/packages/chef-client/chef-client-14.3.37-1-x64.msi /qn /norestart]"
	if have, want := actualMsi, expectedMsi; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	//
	// Exe
	//
	exeItem.DisplayName = statusInstallNoError
	// Run Update
	actualExe := Update(exeItem)
	// Check the result
	expectedExe := "[testdata/packages/chef-client/chef-client-14.3.37-1-x64.exe /L=1033 /S]"
	if have, want := actualExe, expectedExe; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

	//
	// Ps1
	//
	ps1Item.DisplayName = statusInstallNoError
	// Run Update
	actualPs1 := Update(ps1Item)
	// Check the result
	expectedPs1 := "[system32/WindowsPowershell/v1.0/powershell.exe -NoProfile -NoLogo -NonInteractive -WindowStyle Normal -ExecutionPolicy Bypass -File testdata/packages/chef-client/chef-client-14.3.37-1-x64.ps1]"
	if have, want := actualPs1, expectedPs1; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

}

// TestUpdateStatusError verifies that Update returns if status check fails
func TestUpdateStatusError(t *testing.T) {

	// Run the msi installer with this status bypass to trigger an error
	msiItem.DisplayName = statusInstallError
	// Run Update
	actualOutput := Update(msiItem)
	// Check the result
	expectedOutput := "Unable to check status: testing _gorilla_dev_install_error_"
	if have, want := actualOutput, expectedOutput; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

}

// TestUpdateStatusFalse verifies that Update returns if status check is false
func TestUpdateStatusFalse(t *testing.T) {

	// Run the msi installer with this status bypass to make status return dalse
	msiItem.DisplayName = statusNoInstallNoError
	// Run Update
	actualOutput := Update(msiItem)
	// Check the result
	expectedOutput := "Update not needed"
	if have, want := actualOutput, expectedOutput; have != want {
		t.Errorf("\n-----\nhave\n%s\nwant\n%s\n-----", have, want)
	}

}
