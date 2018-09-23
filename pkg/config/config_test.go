package config

import (
	"flag"
	"reflect"
	"testing"
)

// TestGet tests that the configuration is retrieved and parsed properly
func TestGet(t *testing.T) {
	// Define what we expect in a successful test
	expected := Object{
		URL:       "https://example.com/gorilla/",
		Manifest:  "example_manifest",
		Catalog:   "example_catalog",
		CachePath: "c:/cpe/gorilla/cache",
		Verbose:   false,
		Debug:     false,
		AuthUser:  "johnny",
		AuthPass:  "pizza",
	}

	// Pass our test data as a argument
	testArg := []string{"-config", "testdata/test_config.yaml"}
	flag.CommandLine.Parse(testArg)

	// Run the actual code
	Get()

	// Compare the result with our expectations
	structsMatch := reflect.DeepEqual(expected, Config)

	if !structsMatch {
		t.Errorf("\n\nExpected:\n\n%#v\n\nReceived:\n\n %#v", expected, Config)
	}
}
