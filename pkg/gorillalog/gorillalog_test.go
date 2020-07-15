package gorillalog

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/1dustindavis/gorilla/pkg/config"
)

// TestNewLog tests the creation of the log and its directory
func TestNewLog(t *testing.T) {
	// Set up a place for test data
	tmpDir := filepath.Join(os.Getenv("TMPDIR"), "gorillalog")

	cfg := config.Configuration{
		AppDataPath: tmpDir,
		Debug:       false,
		Verbose:     false,
	}

	defer func() {
		// Clean up when we are done
		os.RemoveAll(tmpDir)
	}()

	// Run the function
	NewLog(cfg)

	// Check values
	logDir := tmpDir
	logFile := filepath.Join(tmpDir, "gorilla.log")
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		fmt.Println(err)
		t.Errorf("Log Directory not created: %s", logDir)
	}
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Errorf("Log File not created: %s", logFile)
	}
}

// ExampleDebug_off tests the output of a log sent to DEBUG while config.Debug is false
func ExampleDebug_off() {
	// Set up what we expect
	logString := "Debug String!"

	// Run the function without debug
	debug = false
	Debug(logString)

	// Output:
}

// ExampleDebug_on tests the output of a log sent to DEBUG while config.Debug is true
func ExampleDebug_on() {
	// Set up what we expect
	logString := "Debug String!"

	// Run the function with debug
	debug = true
	Debug(logString)

	// Output:
	// Debug String!
}

// ExampleInfo_verbose_off tests the output of a log sent to INFO while config.Verbose is false
func ExampleInfo_verbose_off() {
	// Set up what we expect
	logString := "Info String!"

	// Run the function without verbose
	verbose = false

	Info(logString)
	// Output:
}

// ExampleInfoVerboseOn tests the output of a log sent to INFO while config.Verbose is true
func ExampleInfo_verbose_on() {
	// Set up what we expect
	logString := "Info String!"

	// Run the function with verbose
	verbose = true

	Info(logString)
	// Output:
	// Info String!
}

// ExampleWarn tests the output of a log sent to WARN
func ExampleWarn() {
	// Set up what we expect
	logString := "Warn String!"

	// Run the function
	Warn(logString)
	// Output:
	// Warn String!
}

// ExampleError tests the output of a log sent to ERROR
func ExampleError() {
	// Set up what we expect
	logString := "Error String!"

	// Prepare to recover from a panic
	defer func() {
		recover()
	}()

	// Run the function
	Error(logString)
	// Output:
}
