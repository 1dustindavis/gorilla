package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestGet tests that the configuration is retrieved and parsed properly.
func TestGet(t *testing.T) {
	expected := Configuration{
		URL:             "https://example.com/gorilla/",
		URLPackages:     "https://example.com/gorilla/",
		Manifest:        "example_manifest",
		LocalManifests:  []string{"example_local_manifest", filepath.Clean("c:/cpe/gorilla/service-manifest.yaml")},
		Catalogs:        []string{"example_catalog"},
		AppDataPath:     filepath.Clean("c:/cpe/gorilla/"),
		Verbose:         true,
		Debug:           true,
		CheckOnly:       true,
		AuthUser:        "johnny",
		AuthPass:        "pizza",
		CachePath:       filepath.Clean("c:/cpe/gorilla/cache"),
		ServiceMode:     false,
		ServiceCommand:  "",
		ServiceInstall:  false,
		ServiceRemove:   false,
		ServiceStart:    false,
		ServiceStop:     false,
		ServiceStatus:   false,
		ServiceName:     "gorilla",
		ServiceInterval: "1h",
		ServicePipeName: "gorilla-service",
		ConfigPath:      "testdata/test_config.yaml",
	}

	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	os.Args = []string{"gorilla.exe", "-config", "testdata/test_config.yaml"}

	cfg := Get()
	if !reflect.DeepEqual(expected, cfg) {
		t.Errorf("\n\nExpected:\n\n%#v\n\nReceived:\n\n%#v", expected, cfg)
	}
}

func TestParseArguments(t *testing.T) {
	expectedConfig := `.\fake.yaml`
	expectedVerbose := true
	expectedDebug := true
	expectedCheckOnly := true

	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	os.Args = []string{"gorilla.exe", "--verbose", "--debug", "--checkonly", "--config", `.\fake.yaml`}

	configArg, verboseArg, debugArg, checkonlyArg := parseArguments()
	if have, want := configArg, expectedConfig; have != want {
		t.Errorf("have %s, want %s", have, want)
	}
	if have, want := checkonlyArg, expectedCheckOnly; have != want {
		t.Errorf("have %v, want %v", have, want)
	}
	if have, want := verboseArg, expectedVerbose; have != want {
		t.Errorf("have %v, want %v", have, want)
	}
	if have, want := debugArg, expectedDebug; have != want {
		t.Errorf("have %v, want %v", have, want)
	}
}

func Example() {
	origExit := osExit
	defer func() { osExit = origExit }()
	osExit = func(code int) { _ = code }

	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	os.Args = []string{"gorilla.exe", "--help"}

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
	// -s, -service        run Gorilla as a Windows service
	// -S, -servicecmd     send a command to a running Gorilla service (ListOptionalInstalls|InstallItem:itemName|RemoveItem:itemName|StreamOperationStatus:operationId)
	// -serviceinstall     install Gorilla as a Windows service
	// -serviceremove      remove Gorilla Windows service
	// -servicestart       start Gorilla Windows service
	// -servicestop        stop Gorilla Windows service
	// -servicestatus      show Gorilla Windows service status
	// -h, -help           display this help message
	//
	// Repository administration uses the separate command boundary:
	//   gorilla admin build [--repo <path>]
}
