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
)

// TODO rewrite with io.multiwriter
// Something like this?
// logOutput := io.MultiWriter(os.Stdout, logFile)
// log.SetOutput(logOutput)

// NewLog creates a file and points a new logging instance at it
func NewLog(cfg config.Configuration) {
	logMu.Lock()
	defer logMu.Unlock()

	// Setup a defer function to recover from a panic
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
			os.Exit(1)
		}
	}()

	// Store the verbosity for later use
	debug = cfg.Debug
	verbose = cfg.Verbose
	checkonly = cfg.CheckOnly

	// Skip log if checkonly is active
	if checkonly {
		return
	}

	// Create the log directory
	logPath := filepath.Join(cfg.AppDataPath, "gorilla.log")
	err := os.MkdirAll(filepath.Dir(logPath), 0755)
	if err != nil {
		msg := fmt.Sprint("Unable to create directory:", logPath, err)
		panic(msg)
	}

	// Create the log file
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		msg := fmt.Sprint("Unable to open file:", file, err)
		panic(msg)
	}

	if logFile != nil {
		_ = logFile.Close()
	}
	logFile = file

	// Configure the `log` package to use our file
	log.SetOutput(logFile)

	//  Configure the `log` package to use microsecond resolution
	log.SetFlags(log.Ldate | log.Lmicroseconds)
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
}

// Debug logs a string as DEBUG
// We write to disk if debug is true
func Debug(logStrings ...interface{}) {
	log.SetPrefix("DEBUG: ")
	if debug {
		fmt.Println(logStrings...)
		if checkonly {
			return
		}
		log.Println(logStrings...)
	}
}

// Info logs a string as INFO
// We only print to stdout if verbose is true
func Info(logStrings ...interface{}) {
	log.SetPrefix("INFO: ")
	if verbose {
		fmt.Println(logStrings...)
	}
	if checkonly {
		return
	}
	log.Println(logStrings...)
}

// Warn logs a string as WARN
// We print to stdout and write to disk
func Warn(logStrings ...interface{}) {
	log.SetPrefix("WARN: ")
	fmt.Println(logStrings...)
	if checkonly {
		return
	}
	log.Println(logStrings...)
}

// Error logs a string a ERROR
// We print to stdout, write to disk, and then panic
func Error(logStrings ...interface{}) {
	if checkonly {
		return
	}
	log.SetPrefix("ERROR: ")
	log.Panic(logStrings...)
}
