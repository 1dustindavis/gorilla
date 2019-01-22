package status

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
)

var (
	// store original data to restore after each test
	origExec          = execCommand
	origCachePath     = config.CachePath
	origVerbose       = config.Current.Verbose
	origRegistryItems = RegistryItems

	// fakeRegistryItems provides fake items for testing checkRegistry
	fakeRegistryItems = map[string]RegistryApplication{
		`registryCheckItem`: RegistryApplication{
			Name:    `Registry Check Item`,
			Version: `1.2.0.3`,
		},
		`registryCheckItemOutdated`: RegistryApplication{
			Name:    `Outdated`,
			Version: `33.6.3`,
		},
	}

	// These catalog items provide test data
	pathInstalled = catalog.Item{
		Check: catalog.InstallCheck{
			File: []catalog.FileCheck{{
				Path: `testdata/test_checkPath.msi`,
				Hash: `cc8f5a895f1c500aa3b4ae35f3878595f4587054a32fa6d7e9f46363525c59f9`,
			}},
		},
	}
	pathNotInstalled = catalog.Item{
		Check: catalog.InstallCheck{
			File: []catalog.FileCheck{{
				Path: `testdata/test_checkPath.msi`,
				Hash: `ba7d5a895f1c500aa3b4ae35f3878595f4587054a32fa6d7e9f46363525c59e8`,
			}},
		},
	}
	pathMetadataInstalled = catalog.Item{
		Check: catalog.InstallCheck{
			File: []catalog.FileCheck{{
				Path:        `testdata/test.exe`,
				Version:     `3.2.0.1`,
				ProductName: `Gorilla Test`,
			}},
		},
	}
	pathMetadataOutdated = catalog.Item{
		Check: catalog.InstallCheck{
			File: []catalog.FileCheck{{
				Path:        `testdata/test.exe`,
				Version:     `3.12.0.1`,
				ProductName: `Gorilla Test`,
			}},
		},
	}
	scriptActionNoError = catalog.Item{
		Installer: catalog.InstallerItem{Type: `ps1`},
	}
	scriptNoActionNoError = catalog.Item{
		Installer:   catalog.InstallerItem{Type: `ps1`},
		DisplayName: `scriptNoActionNoError`,
	}
	scriptCheckItem = catalog.Item{
		Check: catalog.InstallCheck{
			Script: `echo "pizza"`,
		},
		DisplayName: `scriptCheckItem`,
	}
	fileCheckItem = catalog.Item{
		Check: catalog.InstallCheck{
			File: []catalog.FileCheck{{
				Path: `testdata/test_checkPath.msi`,
			}},
		},
		DisplayName: `fileCheckItem`,
	}
	registryCheckItem = catalog.Item{
		Check: catalog.InstallCheck{
			Registry: catalog.RegCheck{
				Version: `1.2.0.3`,
				Name:    `Registry Check Item`,
			},
		},
		DisplayName: `registryCheckItem`,
	}
	registryCheckItemNotInstalled = catalog.Item{
		Check: catalog.InstallCheck{
			Registry: catalog.RegCheck{
				Version: `33.8.3`,
				Name:    `Not Installed`,
			},
		},
		DisplayName: `registryCheckItem`,
	}
	registryCheckItemOutdated = catalog.Item{
		Check: catalog.InstallCheck{
			Registry: catalog.RegCheck{
				Version: `33.12.0`,
				Name:    `Outdated`,
			},
		},
		DisplayName: `registryCheckItem`,
	}
	noCheckItem = catalog.Item{
		DisplayName: `noCheckItem`,
	}

	// Define different options to bypass status checks during tests
	statusActionNoError   = `_gorilla_dev_action_noerror_`
	statusNoActionNoError = `_gorilla_dev_noaction_noerror_`
)

// check if a slice contains a string
func sliceContains(s []string, e string) bool {
	for _, a := range s {
		if strings.Contains(a, e) {
			return true
		}
	}
	return false
}

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
	if sliceContains(os.Args[3:], statusActionNoError) {
		os.Exit(0)
	}
	if sliceContains(os.Args[3:], statusNoActionNoError) {
		os.Exit(1)
	}
	os.Exit(0)
}

// TestCheckRegistry validates that the registry entries are checked properly
func TestCheckRegistry(t *testing.T) {
	// Override execCommand with our fake version
	RegistryItems = fakeRegistryItems
	defer func() {
		RegistryItems = origRegistryItems
	}()

	// install

	// Run checkRegistry with `registryCheckItem` as an `install`
	// We expect no action needed; Only error if action needed is true
	actionNeeded, _ := checkRegistry(registryCheckItem, "install")
	if actionNeeded {
		t.Errorf("actionNeeded: %v; Expected checkRegistry to return false", actionNeeded)
	}

	// Run checkRegistry with `registryCheckItemNotInstalled` as an `install`
	// We expect action is needed; Only error if action needed is false
	actionNeeded, _ = checkRegistry(registryCheckItemNotInstalled, "install")
	if !actionNeeded {
		t.Errorf("actionNeeded: %v; Expected checkRegistry to return true", actionNeeded)
	}

	// Run checkRegistry with `registryCheckItemOutdated` as an `install`
	// We expect action is needed; Only error if action needed is false
	actionNeeded, _ = checkRegistry(registryCheckItemOutdated, "install")
	if !actionNeeded {
		t.Errorf("actionNeeded: %v; Expected checkRegistry to return true", actionNeeded)
	}

	// uninstall

	// Run checkRegistry with `registryCheckItem` as an `uninstall`
	// We expect action is needed; Only error if action needed is false
	actionNeeded, _ = checkRegistry(registryCheckItem, "uninstall")
	if !actionNeeded {
		t.Errorf("actionNeeded: %v; Expected checkRegistry to return true", actionNeeded)
	}

	// Run checkRegistry with `registryCheckItemNotInstalled` as an `uninstall`
	// We expect no action needed; Only error if action needed is true
	actionNeeded, _ = checkRegistry(registryCheckItemNotInstalled, "uninstall")
	if actionNeeded {
		t.Errorf("actionNeeded: %v; Expected checkRegistry to return false", actionNeeded)
	}

	// update

	// Run checkRegistry with `registryCheckItem` as an `update`
	// We expect no action needed; Only error if action needed is true
	actionNeeded, _ = checkRegistry(registryCheckItem, "update")
	if actionNeeded {
		t.Errorf("actionNeeded: %v; Expected checkRegistry to return false", actionNeeded)
	}

	// Run checkRegistry with `registryCheckItemNotInstalled` as an `update`
	// We expect no action needed; Only error if action needed is true
	actionNeeded, _ = checkRegistry(registryCheckItemNotInstalled, "update")
	if actionNeeded {
		t.Errorf("actionNeeded: %v; Expected checkRegistry to return false", actionNeeded)
	}

	// Run checkRegistry with `registryCheckItemOutdated` as an `update`
	// We expect action is needed; Only error if action needed is false
	actionNeeded, _ = checkRegistry(registryCheckItemOutdated, "update")
	if !actionNeeded {
		t.Errorf("actionNeeded: %v; Expected checkRegistry to return true", actionNeeded)
	}

}

// TestCheckScript validates that a script is properly written disk, ran, and then deleted
// and the status is retreived properly.
func TestCheckScript(t *testing.T) {
	// Override execCommand with our fake version
	execCommand = fakeExecCommand
	// Override the cachepath to use our test directory
	config.CachePath = "testdata/"
	defer func() {
		execCommand = origExec
		config.CachePath = origCachePath
	}()

	// Set cachepath and run checkScript for scriptActionNoError
	config.CachePath = fmt.Sprintf("testdata/%s/", statusActionNoError)
	actionNeeded, err := checkScript(scriptActionNoError)
	if !actionNeeded || err != nil {
		fmt.Printf("action: %v; error: %v\n", actionNeeded, err)
		t.Errorf("Expected checkScript to action and no error")
	}

	// Set cachepath and run checkScript for scriptNoActionNoError
	config.CachePath = fmt.Sprintf("testdata/%s/", statusNoActionNoError)
	actionNeeded, err = checkScript(scriptNoActionNoError)
	if actionNeeded || err != nil {
		fmt.Printf("action: %v; error: %v\n", actionNeeded, err)
		t.Errorf("Expected checkScript to return no action and no error")
	}

}

// TestCheckPath validates that the status of a path is checked correctly
func TestCheckPath(t *testing.T) {
	// Override the cachepath to use our test directory
	config.CachePath = "testdata/"
	defer func() {
		config.CachePath = origCachePath
	}()

	// Run checkPath for pathInstalled
	// We expect action is not needed; Only error if action needed is true
	actionNeeded, err := checkPath(pathInstalled)
	if err != nil {
		t.Errorf("checkPath failed: %v", err)
	}
	if actionNeeded {
		t.Errorf("actionNeeded: %v; Expected checkPath to return false", actionNeeded)
	}

	// Run checkPath for pathNotInstalled
	// We expect action is needed; Only error if actionNeeded is false
	actionNeeded, err = checkPath(pathNotInstalled)
	if err != nil {
		t.Error(err)
	}
	if !actionNeeded {
		t.Errorf("actionNeeded: %v; Expected checkPath to return true", actionNeeded)
	}

	// Run checkPath for pathMetadataInstalled
	// We expect action is not needed; Only error if actionNeeded is true
	actionNeeded, err = checkPath(pathMetadataInstalled)
	if err != nil {
		t.Error(err)
	}
	if actionNeeded {
		t.Errorf("actionNeeded: %v; Expected checkPath to return true", actionNeeded)
	}

	// Run checkPath for pathMetadataOutdated
	// We expect action is needed; Only error if actionNeeded is false
	actionNeeded, err = checkPath(pathMetadataOutdated)
	if err != nil {
		t.Error(err)
	}
	if !actionNeeded {
		t.Errorf("actionNeeded: %v; Expected checkPath to return true", actionNeeded)
	}

}

// ExampleCheckStatus_script validates that a script check is ran
func ExampleCheckStatus_script() {
	// Override execCommand with our fake version
	execCommand = fakeExecCommand
	// Override the cachepath to use our test directory
	config.CachePath = "testdata/"
	// Override the verbose setting
	config.Current.Verbose = true
	defer func() {
		execCommand = origExec
		config.CachePath = origCachePath
		config.Current.Verbose = origVerbose
	}()

	// Run CheckStatus with an item that has a script check
	CheckStatus(scriptCheckItem, "install")

	// Output:
	// Checking status via Script: scriptCheckItem
}

// ExampleCheckStatus_file validates that a file check is ran
func ExampleCheckStatus_file() {
	// Override execCommand with our fake version
	execCommand = fakeExecCommand
	// Override the cachepath to use our test directory
	config.CachePath = "testdata/"
	// Override the verbose setting
	config.Current.Verbose = true
	defer func() {
		execCommand = origExec
		config.CachePath = origCachePath
		config.Current.Verbose = origVerbose
	}()

	// Run CheckStatus with an item that has a script check
	CheckStatus(fileCheckItem, "install")

	// Output:
	// Checking status via File: fileCheckItem
}

// ExampleCheckStatus_registry validates that a registry check is ran
func ExampleCheckStatus_registry() {
	// Override execCommand with our fake version
	execCommand = fakeExecCommand
	// Override the cachepath to use our test directory
	config.CachePath = "testdata/"
	// Override the verbose setting
	config.Current.Verbose = true
	defer func() {
		execCommand = origExec
		config.CachePath = origCachePath
		config.Current.Verbose = origVerbose
	}()

	// Run CheckStatus with an item that has a script check
	CheckStatus(registryCheckItem, "install")

	// Output:
	// Checking status via Registry: registryCheckItem
}

// ExampleCheckStatus_none validates that no check is ran
func ExampleCheckStatus_none() {
	// Override execCommand with our fake version
	execCommand = fakeExecCommand
	// Override the cachepath to use our test directory
	config.CachePath = "testdata/"
	// Override the verbose setting
	config.Current.Verbose = false
	defer func() {
		execCommand = origExec
		config.CachePath = origCachePath
		config.Current.Verbose = origVerbose
	}()

	// Run CheckStatus with an item that has a script check
	CheckStatus(noCheckItem, "install")

	// Output:
	// Not enough data to check the current status: noCheckItem
}
