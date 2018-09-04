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

// Store the original values, before we override
var (
	origVerbose     = config.Verbose
	origDebug       = config.Debug
	origProgramData = config.GorillaData
)

func TestNewLog(t *testing.T) {
	// Set up a place for test data
	tmpDir := filepath.Join(os.Getenv("TMPDIR"), "gorillalog")
	config.GorillaData = tmpDir

	// Clean up when we are done
	defer func() {
		// Clean up
		config.GorillaData = origProgramData
		os.RemoveAll(tmpDir)
	}()

	// Run the function
	NewLog()

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

func TestDebug(t *testing.T) {
	// Set the output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() {
		log.SetOutput(os.Stderr)
	}()

	// Set up what we want
	prefix := "DEBUG: "
	now := time.Now().Format("2006/01/02 15:04:05 ")
	logString := "Debug String!"
	expected := fmt.Sprint(prefix, now, logString)

	// Run the function
	config.Debug = true
	Debug(logString)
	config.Debug = origDebug

	result := strings.TrimSpace(buf.String())

	// Check out result
	if have, want := result, expected; have != want {
		t.Errorf("-----\nhave %s\nwant %s\n-----", have, want)
	}
}

func ExampleDebugOff() {
	// Set up what we expect
	logString := "Debug String!"

	// Run the function without debug
	config.Debug = false
	Debug(logString)
	config.Debug = origDebug
	// Output:
}

func ExampleDebugOn() {
	// Set up what we expect
	logString := "Debug String!"

	// Run the function with debug
	config.Debug = true
	Debug(logString)
	config.Debug = origDebug
	// Output:
	// Debug String!
}

func TestInfo(t *testing.T) {
	// Set the output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() {
		log.SetOutput(os.Stderr)
	}()

	// Set up what we want
	prefix := "INFO: "
	now := time.Now().Format("2006/01/02 15:04:05 ")
	logString := "Info String!"
	expected := fmt.Sprint(prefix, now, logString)

	// Run the function
	Info(logString)

	result := strings.TrimSpace(buf.String())

	// Check out result
	if have, want := result, expected; have != want {
		t.Errorf("-----\nhave %s\nwant %s\n-----", have, want)
	}
}

func ExampleInfoVerboseOff() {
	// Set up what we expect
	logString := "Info String!"

	// Run the function without verbose
	config.Verbose = false
	Info(logString)
	config.Verbose = origVerbose
	// Output:
}

func ExampleInfoVerboseOn() {
	// Set up what we expect
	logString := "Info String!"

	// Run the function with verbose
	config.Verbose = true
	Info(logString)
	config.Verbose = origVerbose
	// Output:
	// Info String!
}

func TestWarn(t *testing.T) {
	// Set the output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() {
		log.SetOutput(os.Stderr)
	}()

	// Set up what we want
	prefix := "WARN: "
	now := time.Now().Format("2006/01/02 15:04:05 ")
	logString := "Warn String!"
	expected := fmt.Sprint(prefix, now, logString)

	// Run the function
	Warn(logString)

	result := strings.TrimSpace(buf.String())

	// Check out result
	if have, want := result, expected; have != want {
		t.Errorf("-----\nhave %s\nwant %s\n-----", have, want)
	}
}

func ExampleWarn() {
	// Set up what we expect
	logString := "Warn String!"

	// Run the function without verbose
	Warn(logString)
	// Output:
	// Warn String!
}

func TestError(t *testing.T) {
	// Set the output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() {
		log.SetOutput(os.Stderr)
	}()

	// Set up what we want
	prefix := "ERROR: "
	now := time.Now().Format("2006/01/02 15:04:05 ")
	logString := "Error String!"
	expected := fmt.Sprint(prefix, now, logString)

	// Prepare to recover from a panic
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Error didnt panic")
		}
	}()

	// Run the function
	Error(logString)

	result := strings.TrimSpace(buf.String())

	// Check out result
	if have, want := result, expected; have != want {
		t.Errorf("-----\nhave %s\nwant %s\n-----", have, want)
	}
}

func ExampleError() {
	// Set up what we expect
	logString := "Error String!"

	// Prepare to recover from a panic
	defer func() {
		recover()
	}()

	// Run the function without verbose
	Error(logString)
	// Output:
}
