package config

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"

	"github.com/1dustindavis/gorilla/pkg/report"
	"github.com/1dustindavis/gorilla/pkg/version"
)

var (
	// CachePath is the path to a location for temporary storage
	CachePath string
)

// Object to store our configuration
type Object struct {
	URL           string `yaml:"url"`
	Manifest      string `yaml:"manifest"`
	Catalog       string `yaml:"catalog"`
	AppDataPath   string `yaml:"app_data_path"`
	Verbose       bool   `yaml:"verbose,omitempty"`
	Debug         bool   `yaml:"debug,omitempty"`
	AuthUser      string `yaml:"auth_user,omitempty"`
	AuthPass      string `yaml:"auth_pass,omitempty"`
	TLSAuth       bool   `yaml:"tls_auth,omitempty"`
	TLSClientCert string `yaml:"tls_client_cert,omitempty"`
	TLSClientKey  string `yaml:"tls_client_key,omitempty"`
	TLSServerCert string `yaml:"tls_server_cert,omitempty"`
}

// Define flag defaults
var (
	aboutArg       bool
	aboutDefault   = false
	configArg      string
	configDefault  = filepath.Join(os.Getenv("ProgramData"), "gorilla/config.yaml")
	debugArg       bool
	debugDefault   = false
	helpArg        bool
	helpDefault    = false
	verboseArg     bool
	verboseDefault = false
	versionArg     bool
	versionDefault = false
)

const usage = `
Gorilla - Munki-like Application Management for Windows
https://github.com/1dustindavis/gorilla

Usage: gorilla.exe [options]

Options:
-c, -config         path to configuration file in yaml format
-v, -verbose        enable verbose output
-d, -debug          enable debug output
-a, -about          displays the version number and other build info
-V, -version        display the version number
-h, -help           display this help message

`

func init() {
	// Define flag names and defaults here

	// About
	flag.BoolVar(&aboutArg, "about", aboutDefault, "")
	flag.BoolVar(&aboutArg, "a", aboutDefault, "")
	// Config
	flag.StringVar(&configArg, "config", configDefault, "")
	flag.StringVar(&configArg, "c", configDefault, "")
	// Debug
	flag.BoolVar(&debugArg, "debug", debugDefault, "")
	flag.BoolVar(&debugArg, "d", debugDefault, "")
	// Help
	flag.BoolVar(&helpArg, "help", helpDefault, "")
	flag.BoolVar(&helpArg, "h", helpDefault, "")
	// Verbose
	flag.BoolVar(&verboseArg, "verbose", verboseDefault, "")
	flag.BoolVar(&verboseArg, "v", verboseDefault, "")
	// Version
	flag.BoolVar(&versionArg, "version", versionDefault, "")
	flag.BoolVar(&versionArg, "V", versionDefault, "")
}

// Use a fake function so we can override when testing
var osExit = os.Exit

func parseArguments() (string, bool, bool) {
	// Get the command line args
	flag.Parse()
	if helpArg {
		version.Print()
		fmt.Print(usage)
		osExit(0)
	}
	if versionArg {
		version.Print()
		osExit(0)
	}
	if aboutArg {
		version.PrintFull()
		osExit(0)
	}

	return configArg, verboseArg, debugArg
}

// Current is a global struct to store our configuration in
var Current Object

// Get retrieves and then stores the local configuration
func Get() {

	configPath, verbose, debug := parseArguments()

	// Get the config at configpath and return a config.Object
	configFile, err := ioutil.ReadFile(configPath)
	if err != nil {
		fmt.Println("Unable to read configuration file: ", err)
		os.Exit(1)
	}
	err = yaml.Unmarshal(configFile, &Current)
	if err != nil {
		fmt.Println("Unable to parse yaml configuration: ", err)
		os.Exit(1)
	}
	// If URL wasnt provided, exit
	if Current.URL == "" {
		fmt.Println("Invalid configuration - URL: ", err)
		os.Exit(1)
	}
	// If Manifest wasnt provided, exit
	if Current.Manifest == "" {
		fmt.Println("Invalid configuration - Manifest: ", err)
		os.Exit(1)
	}
	// If AppDataPath wasn't provided, configure a default
	if Current.AppDataPath == "" {
		Current.AppDataPath = filepath.Join(os.Getenv("ProgramData"), "gorilla/")
	}
	// Set the verbosity
	if verbose == true && !Current.Verbose {
		Current.Verbose = true
	}
	// Set the debug and verbose
	if debug == true && !Current.Debug {
		Current.Debug = true
		Current.Verbose = true
	}

	// Set the cache path
	CachePath = filepath.Join(Current.AppDataPath, "cache")

	// Add to GorillaReport
	report.Items["Manifest"] = Current.Manifest
	report.Items["Catalog"] = Current.Catalog

	return
}
