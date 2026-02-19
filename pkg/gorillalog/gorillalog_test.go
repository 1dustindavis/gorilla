package gorillalog

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/config"
)

// TestNewLog tests the creation of the log and its directory
func TestNewLog(t *testing.T) {
	Close()
	log.SetOutput(os.Stdout)

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
		Close()
		log.SetOutput(os.Stdout)
	}()

	// Run the function
	if err := NewLog(cfg); err != nil {
		t.Fatalf("NewLog failed: %v", err)
	}

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

func TestNewLogRotatesOversizedFileOnStartup(t *testing.T) {
	Close()
	log.SetOutput(os.Stdout)

	tmpDir := t.TempDir()
	originalMax := logMaxSizeBytes
	logMaxSizeBytes = 16
	t.Cleanup(func() {
		logMaxSizeBytes = originalMax
		Close()
		log.SetOutput(os.Stdout)
	})

	activePath := filepath.Join(tmpDir, "gorilla.log")
	if err := os.WriteFile(activePath, []byte(strings.Repeat("x", 64)), 0644); err != nil {
		t.Fatalf("seed log file: %v", err)
	}

	cfg := config.Configuration{
		AppDataPath: tmpDir,
	}
	if err := NewLog(cfg); err != nil {
		t.Fatalf("NewLog failed: %v", err)
	}

	if _, err := os.Stat(activePath + ".1"); err != nil {
		t.Fatalf("expected rotated backup log, got error: %v", err)
	}
}

func TestInfoRotatesOversizedActiveFileBeforeWrite(t *testing.T) {
	Close()
	log.SetOutput(os.Stdout)

	tmpDir := t.TempDir()
	originalMax := logMaxSizeBytes
	logMaxSizeBytes = 40
	t.Cleanup(func() {
		logMaxSizeBytes = originalMax
		Close()
		log.SetOutput(os.Stdout)
	})

	cfg := config.Configuration{
		AppDataPath: tmpDir,
	}
	if err := NewLog(cfg); err != nil {
		t.Fatalf("NewLog failed: %v", err)
	}

	Info(strings.Repeat("A", 128))
	Info("second line triggers size check")

	backupPath := filepath.Join(tmpDir, "gorilla.log.1")
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("expected rotated backup log, got error: %v", err)
	}
}

func TestRotateLogIfNeededPreservesPreviousBackupWhenRenameFails(t *testing.T) {
	tmpDir := t.TempDir()
	activePath := filepath.Join(tmpDir, "gorilla.log")
	backupPath := activePath + ".1"

	if err := os.WriteFile(activePath, []byte(strings.Repeat("a", 128)), 0644); err != nil {
		t.Fatalf("write active log: %v", err)
	}
	if err := os.WriteFile(backupPath, []byte("previous-backup"), 0644); err != nil {
		t.Fatalf("write backup log: %v", err)
	}

	originalMax := logMaxSizeBytes
	logMaxSizeBytes = 32
	t.Cleanup(func() { logMaxSizeBytes = originalMax })

	originalRename := osRename
	osRename = func(oldpath, newpath string) error {
		if oldpath == activePath && newpath == backupPath {
			return errors.New("forced rename failure")
		}
		return originalRename(oldpath, newpath)
	}
	t.Cleanup(func() { osRename = originalRename })

	if err := rotateLogIfNeeded(activePath); err == nil {
		t.Fatalf("expected rotateLogIfNeeded to fail when active rename fails")
	}

	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("expected backup to still exist: %v", err)
	}
	if string(backupContent) != "previous-backup" {
		t.Fatalf("expected previous backup content to be preserved, got %q", string(backupContent))
	}
}

func TestRotateLogIfNeededReturnsStatErrorWithoutPanicking(t *testing.T) {
	originalStat := osStat
	osStat = func(string) (os.FileInfo, error) {
		return nil, errors.New("forced stat failure")
	}
	t.Cleanup(func() { osStat = originalStat })

	if err := rotateLogIfNeeded("/tmp/does-not-matter.log"); err == nil {
		t.Fatalf("expected rotateLogIfNeeded to return stat error")
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
