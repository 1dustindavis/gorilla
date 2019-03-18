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
	// CachePath is a directory we will use for temporary storage
	cachePath string

	// Define flag defaults
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

	// Use a fake function so we can override when testing
	osExit = os.Exit
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

// Configuration stores all of the possible parameters a config file could contain
type Configuration struct {
	URL           string   `yaml:"url"`
	URLPackages   string   `yaml:"url_packages"`
	Manifest      string   `yaml:"manifest"`
	Catalogs      []string `yaml:"catalogs"`
	AppDataPath   string   `yaml:"app_data_path"`
	Verbose       bool     `yaml:"verbose,omitempty"`
	Debug         bool     `yaml:"debug,omitempty"`
	AuthUser      string   `yaml:"auth_user,omitempty"`
	AuthPass      string   `yaml:"auth_pass,omitempty"`
	TLSAuth       bool     `yaml:"tls_auth,omitempty"`
	TLSClientCert string   `yaml:"tls_client_cert,omitempty"`
	TLSClientKey  string   `yaml:"tls_client_key,omitempty"`
	TLSServerCert string   `yaml:"tls_server_cert,omitempty"`
	CachePath     string
}

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

// Get retrieves and parses the config file and returns a Configuration struct and any errors
func Get() Configuration {
	var cfg Configuration

	// Parse any arguments that may have been passed
	configPath, verbose, debug := parseArguments()

	// Read the config file
	configFile, err := ioutil.ReadFile(configPath)
	if err != nil {
		fmt.Println("Unable to read configuration file: ", err)
		os.Exit(1)
	}

	// Parse the config into a struct
	err = yaml.Unmarshal(configFile, &cfg)
	if err != nil {
		fmt.Println("Unable to parse yaml configuration: ", err)
		os.Exit(1)
	}

	// If Manifest wasnt provided, exit
	if cfg.Manifest == "" {
		fmt.Println("Invalid configuration - Manifest: ", err)
		os.Exit(1)
	}

	// If URL wasnt provided, exit
	if cfg.URL == "" {
		fmt.Println("Invalid configuration - URL: ", err)
		os.Exit(1)
	}

	// If URLPackages wasn't provided, use the repo URL
	if cfg.URLPackages == "" {
		cfg.URLPackages = cfg.URL
	}

	// If AppDataPath wasn't provided, configure a default
	if cfg.AppDataPath == "" {
		cfg.AppDataPath = filepath.Join(os.Getenv("ProgramData"), "gorilla/")
	} else {
		cfg.AppDataPath = filepath.Clean(cfg.AppDataPath)
	}

	// Set the verbosity
	if verbose && !cfg.Verbose {
		cfg.Verbose = true
	}

	// Set the debug and verbose
	if debug && !cfg.Debug {
		cfg.Debug = true
		cfg.Verbose = true
	}

	// Set the cache path
	cfg.CachePath = filepath.Join(cfg.AppDataPath, "cache")

	// Add to GorillaReport
	report.Items["Manifest"] = cfg.Manifest
	report.Items["Catalog"] = cfg.Catalogs

	return cfg
}
