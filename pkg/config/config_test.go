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
		URL:             "https://example.com/gorilla/",
		URLPackages:     "https://example.com/gorilla/",
		Manifest:        "example_manifest",
		LocalManifests:  []string{"example_local_manifest"},
		Catalogs:        []string{"example_catalog"},
		RepoPath:        filepath.Clean("c:/repo/gorilla"),
		AppDataPath:     filepath.Clean("c:/cpe/gorilla/"),
		Verbose:         true,
		Debug:           true,
		CheckOnly:       true,
		BuildArg:        false,
		ImportArg:       "",
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

func TestGetBuildModeWithoutManifestOrURL(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "build_config.yaml")
	configYAML := []byte(`
url: https://example.com/gorilla/
manifest: example_manifest
app_data_path: c:/cpe/gorilla/
repo_path: c:/repo/gorilla
`)
	if err := os.WriteFile(configPath, configYAML, 0644); err != nil {
		t.Fatal(err)
	}

	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	origBuildArg := buildArg
	origImportArg := importArg
	defer func() {
		buildArg = origBuildArg
		importArg = origImportArg
	}()

	// Flag parsing is process-global in this package; set mode flags directly for test stability.
	buildArg = true
	importArg = ""
	os.Args = []string{"gorilla.exe", "--build", "--config", configPath}
	cfg := Get()

	if !cfg.BuildArg {
		t.Fatalf("expected BuildArg to be true")
	}
	if cfg.ImportArg != "" {
		t.Fatalf("expected ImportArg to be empty")
	}
	if cfg.RepoPath != filepath.Clean("c:/repo/gorilla") {
		t.Fatalf("unexpected RepoPath: %s", cfg.RepoPath)
	}
}

// TestParseArguments tests if flag is parsed correctly
func TestParseArguments(t *testing.T) {

	// Set our expectations
	expectedConfig := `.\fake.yaml`
	expectedVerbose := true
	expectedDebug := true
	expectedCheckOnly := true
	expectedBuild := true
	expectedImport := `.\foo.exe`

	// Save the original arguments
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// Override with our input
	os.Args = []string{"gorilla.exe", "--verbose", "--debug", "--checkonly", "--build", "--import", `.\foo.exe`, "--config", `.\fake.yaml`}

	// Run code
	configArg, verboseArg, debugArg, checkonlyArg, buildArg, importArg := parseArguments()

	// Compare config
	if have, want := configArg, expectedConfig; have != want {
		t.Errorf("have %s, want %s", have, want)
	}

	// Compare checkonly
	if have, want := checkonlyArg, expectedCheckOnly; have != want {
		t.Errorf("have %v, want %v", have, want)
	}

	// Compare build
	if have, want := buildArg, expectedBuild; have != want {
		t.Errorf("have %v, want %v", have, want)
	}

	// Compare import
	if have, want := importArg, expectedImport; have != want {
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
	_, _, _, _, _, _ = parseArguments()

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
	// -b, -build          build catalog files from package-info files
	// -i, -import         create a package-info file from an installer package
	// -v, -verbose        enable verbose output
	// -d, -debug          enable debug output
	// -a, -about          displays the version number and other build info
	// -V, -version        display the version number
	// -s, -service        run Gorilla as a Windows service
	// -S, -servicecmd     send a command to a running Gorilla service (run|install:item1,item2|remove:item1|get-service-manifest|get-optional-items)
	// -serviceinstall     install Gorilla as a Windows service
	// -serviceremove      remove Gorilla Windows service
	// -servicestart       start Gorilla Windows service
	// -servicestop        stop Gorilla Windows service
	// -servicestatus      show Gorilla Windows service status
	// -h, -help           display this help message
}
