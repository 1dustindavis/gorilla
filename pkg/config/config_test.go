package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestGet tests that the configuration is retrieved and parsed properly
func TestGet(t *testing.T) {
	// Define what we expect in a successful test
	expected := Configuration{
		URL:            "https://example.com/gorilla/",
		URLPackages:    "https://example.com/gorilla/",
		Manifest:       "example_manifest",
		LocalManifests: []string{"example_local_manifest"},
		Catalogs:       []string{"example_catalog"},
		AppDataPath:    filepath.Clean("c:/cpe/gorilla/"),
		Verbose:        true,
		Debug:          true,
		CheckOnly:      true,
		AuthUser:       "johnny",
		AuthPass:       "pizza",
		CachePath:      filepath.Clean("c:/cpe/gorilla/cache"),
	}

	// Save the original arguments
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// Override with our input
	os.Args = []string{"gorilla.exe", "-config", "testdata/test_config.yaml"}

	// Run the actual code
	cfg := Get()

	// Compare the result with our expectations
	structsMatch := reflect.DeepEqual(expected, cfg)

	if !structsMatch {
		t.Errorf("\n\nExpected:\n\n%#v\n\nReceived:\n\n%#v", expected, cfg)
	}
}

// TestParseArguments tests if flag is parsed correctly
func TestParseArguments(t *testing.T) {

	// Set our expectations
	expectedConfig := `.\fake.yaml`
	expectedVerbose := true
	expectedDebug := true
	expectedCheckOnly := true

	// Save the original arguments
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// Override with our input
	os.Args = []string{"gorilla.exe", "--verbose", "--debug", "--checkonly", "--config", `.\fake.yaml`}

	// Run code
	configArg, verboseArg, debugArg, checkonlyArg := parseArguments()

	// Compare config
	if have, want := configArg, expectedConfig; have != want {
		t.Errorf("have %s, want %s", have, want)
	}

	// Compare checkonly
	if have, want := checkonlyArg, expectedCheckOnly; have != want {
		t.Errorf("have %v, want %v", have, want)
	}

	// Compare verbose
	if have, want := verboseArg, expectedVerbose; have != want {
		t.Errorf("have %v, want %v", have, want)
	}

	// Compare debug
	if have, want := debugArg, expectedDebug; have != want {
		t.Errorf("have %v, want %v", have, want)
	}
}

// Example tests if help is is parsed properly
func Example() {

	// Save the original osExit
	origExit := osExit
	defer func() { osExit = origExit }()

	// Override with a fake exit
	// var exitCode int
	osExit = func(code int) {
		_ = code
	}

	// Save the original arguments
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// Override with our input
	os.Args = []string{"gorilla.exe", "--help"}

	// Run code, ignoring the return values
	_, _, _, _ = parseArguments()

	// Output:
	// unknown unknown
	//
	// Gorilla - Munki-like Application Management for Windows
	// https://github.com/1dustindavis/gorilla
	//
	// Usage: gorilla.exe [options]
	//
	// Options:
	// -c, -config         path to configuration file in yaml format
	// -C, -checkonly	    enable check only mode
	// -v, -verbose        enable verbose output
	// -d, -debug          enable debug output
	// -a, -about          displays the version number and other build info
	// -V, -version        display the version number
	// -h, -help           display this help message
}
