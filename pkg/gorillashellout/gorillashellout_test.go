package gorillashellout

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"io/ioutil"

	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
)

var (
	// store original data to restore after each test
	origExec = execCommand
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

	actualCmd := RunCommand(testCommand, testArgs)

	// Compare the result with our expectations
	structsMatch := reflect.DeepEqual(expectedCmd, actualCmd)

	if !structsMatch {
		t.Errorf("\nExpected: %#v\nReceived: %#v", expectedCmd, actualCmd)
	}
}

// Example_RunCommand tests the output when running a command in debug
func Example_RunCommand() {
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
	RunCommand(testCmd, testArgs)

	// Output:
	// command: Command Test! [arg1 arg2]
	// Command Output:
	// --------------------
	// [Command Test! arg1 arg2]
	// --------------------
}
