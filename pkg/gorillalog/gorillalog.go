package gorillalog

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/1dustindavis/gorilla/pkg/config"
)

var (
	// Define these config variables at the package scope
	debug   bool
	verbose bool
)

// TODO rewrite with io.multiwriter
// Something like this?
// logOutput := io.MultiWriter(os.Stdout, logFile)
// log.SetOutput(logOutput)

// NewLog creates a file and points a new logging instance at it
func NewLog(cfg config.Configuration) {
	// Setup a defer function to recover from a panic
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered from panic while creating the log file:", r)
			os.Exit(1)
		}
	}()

	// Store the verbosity for later use
	debug = cfg.Debug
	verbose = cfg.Verbose

	// Create the log directory
	logPath := filepath.Join(cfg.AppDataPath, "/gorilla.log")
	err := os.MkdirAll(filepath.Dir(logPath), 0755)
	if err != nil {
		msg := fmt.Sprint("Unable to create directory:", logPath, err)
		panic(msg)
	}

	// Create the log file
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		msg := fmt.Sprint("Unable to open file:", logFile, err)
		panic(msg)
	}

	// Configure the `log` package to use our file
	log.SetOutput(logFile)
}

// Debug logs a string as DEBUG
// We write to disk if debug is true
func Debug(logStrings ...interface{}) {
	log.SetPrefix("DEBUG: ")
	if debug {
		fmt.Println(logStrings...)
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
	log.Println(logStrings...)
}

// Warn logs a string as WARN
// We print to stdout and write to disk
func Warn(logStrings ...interface{}) {
	log.SetPrefix("WARN: ")
	fmt.Println(logStrings...)
	log.Println(logStrings...)
}

// Error logs a string a ERROR
// We print to stdout, write to disk, and then panic
func Error(logStrings ...interface{}) {
	log.SetPrefix("ERROR: ")
	log.Panic(logStrings...)
}
