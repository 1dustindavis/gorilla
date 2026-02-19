package gorillalog

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/1dustindavis/gorilla/pkg/config"
)

var (
	// Define these config variables at the package scope
	debug     bool
	verbose   bool
	checkonly bool
	logMu     sync.Mutex
	logFile   *os.File
	logPath   string

	logMaxSizeBytes int64 = 10 * 1024 * 1024
	osRename              = os.Rename
	osRemove              = os.Remove
	osStat                = os.Stat
)

// TODO rewrite with io.multiwriter
// Something like this?
// logOutput := io.MultiWriter(os.Stdout, logFile)
// log.SetOutput(logOutput)
//
// Diagnostics policy notes:
// - Service operational logs currently write to <app_data_path>/gorilla.log (except checkonly mode).
// - High-volume service trace logging must stay behind debug mode only.
// - Follow-up implementation should add bounded retention/rotation and standardized
//   correlation fields (requestId, operationId, operation, state, result, durationMs).

// NewLog creates a file and points a new logging instance at it.
func NewLog(cfg config.Configuration) error {
	logMu.Lock()
	defer logMu.Unlock()

	// Store the verbosity for later use
	debug = cfg.Debug
	verbose = cfg.Verbose
	checkonly = cfg.CheckOnly

	// Skip log if checkonly is active
	if checkonly {
		return nil
	}

	// Create the log directory
	logPath = filepath.Join(cfg.AppDataPath, "gorilla.log")
	err := os.MkdirAll(filepath.Dir(logPath), 0755)
	if err != nil {
		return fmt.Errorf("unable to create log directory %s: %w", filepath.Dir(logPath), err)
	}

	_ = rotateLogIfNeeded(logPath)

	// Create the log file
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("unable to open log file %s: %w", logPath, err)
	}

	if logFile != nil {
		_ = logFile.Close()
	}
	logFile = file

	// Configure the `log` package to use our file
	log.SetOutput(logFile)

	//  Configure the `log` package to use microsecond resolution
	log.SetFlags(log.Ldate | log.Lmicroseconds)
	return nil
}

// Close releases the active log file handle, if one is open.
func Close() {
	logMu.Lock()
	defer logMu.Unlock()

	if logFile == nil {
		return
	}
	log.SetOutput(io.Discard)
	_ = logFile.Close()
	logFile = nil
	logPath = ""
}

// Debug logs a string as DEBUG
// We write to disk if debug is true
func Debug(logStrings ...interface{}) {
	if debug {
		fmt.Println(logStrings...)
		if checkonly {
			return
		}
		logMu.Lock()
		defer logMu.Unlock()
		rotateCurrentLogIfNeeded()
		log.SetPrefix("DEBUG: ")
		log.Println(logStrings...)
	}
}

// Info logs a string as INFO
// We only print to stdout if verbose is true
func Info(logStrings ...interface{}) {
	if verbose {
		fmt.Println(logStrings...)
	}
	if checkonly {
		return
	}
	logMu.Lock()
	defer logMu.Unlock()
	rotateCurrentLogIfNeeded()
	log.SetPrefix("INFO: ")
	log.Println(logStrings...)
}

// Warn logs a string as WARN
// We print to stdout and write to disk
func Warn(logStrings ...interface{}) {
	fmt.Println(logStrings...)
	if checkonly {
		return
	}
	logMu.Lock()
	defer logMu.Unlock()
	rotateCurrentLogIfNeeded()
	log.SetPrefix("WARN: ")
	log.Println(logStrings...)
}

// Error logs a string a ERROR
// We print to stdout, write to disk, and then panic
func Error(logStrings ...interface{}) {
	if checkonly {
		return
	}
	logMu.Lock()
	defer logMu.Unlock()
	rotateCurrentLogIfNeeded()
	log.SetPrefix("ERROR: ")
	log.Panic(logStrings...)
}

func rotateCurrentLogIfNeeded() {
	if logFile == nil || logPath == "" {
		return
	}
	info, err := logFile.Stat()
	if err != nil || info.Size() < logMaxSizeBytes {
		return
	}
	log.SetOutput(io.Discard)
	_ = logFile.Close()
	logFile = nil
	_ = rotateLogIfNeeded(logPath)
	file, openErr := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if openErr != nil {
		return
	}
	log.SetOutput(file)
	logFile = file
}

func rotateLogIfNeeded(path string) error {
	if logMaxSizeBytes <= 0 {
		return nil
	}
	info, err := osStat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Size() < logMaxSizeBytes {
		return nil
	}

	backupPath := path + ".1"
	backupStashPath := path + ".1.swap"
	if rmErr := osRemove(backupStashPath); rmErr != nil && !os.IsNotExist(rmErr) {
		return rmErr
	}

	hasBackup := false
	if _, statErr := osStat(backupPath); statErr == nil {
		if renameErr := osRename(backupPath, backupStashPath); renameErr != nil {
			return renameErr
		}
		hasBackup = true
	}

	if renameErr := osRename(path, backupPath); renameErr != nil {
		if hasBackup {
			_ = osRename(backupStashPath, backupPath)
		}
		return renameErr
	}

	if hasBackup {
		if rmErr := osRemove(backupStashPath); rmErr != nil && !os.IsNotExist(rmErr) {
			return rmErr
		}
	}

	return nil
}
