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
	// URL of the web server with all of our files
	URL string
	// Manifest is a yaml file with the packages to manage on this node
	Manifest string
	// Catalog is a yaml file with details on the available packages
	Catalog string
	// CachePath is a directory we will use for temporary storage
	CachePath string
	// Verbose is true is we should output more detail than normal
	Verbose bool
	// Debug is true is we should output as much as we can
	Debug bool
	// AuthUser is the username we will use for basic http auth
	AuthUser string
	// AuthPass is the password we will use for basic http auth
	AuthPass string
	// TLSAuth determines if we should use TLS mutual auth
	TLSAuth bool
	// TLSClientCert is the path to the client cert we will use for TLS auth
	TLSClientCert string
	// TLSClientKey is the path to the client key we will use for TLS auth
	TLSClientKey string
	// TLSServerCert is the path to the server cert we will use for TLS auth
	TLSServerCert string
	// GorillaData is the location we store gerneral app data
	GorillaData = filepath.Join(os.Getenv("ProgramData"), "gorilla/")
)

// Object to store our configuration
type Object struct {
	URL           string `yaml:"url"`
	Manifest      string `yaml:"manifest"`
	Catalog       string `yaml:"catalog"`
	CachePath     string `yaml:"cachepath"`
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

func parseArguments() (string, bool, bool) {
	// Get the command line args
	flag.Parse()
	if helpArg {
		version.Print()
		fmt.Print(usage)
		os.Exit(0)
	}
	if versionArg {
		version.Print()
		os.Exit(0)
	}
	if aboutArg {
		version.PrintFull()
		os.Exit(0)
	}

	return configArg, verboseArg, debugArg
}

// Config is a global struct to store our configuration in
var Config Object

// Get retrieves and then stores the local configuration
func Get() {

	configPath, verbose, debug := parseArguments()

	// Get the config at configpath and return a config.Object
	configFile, err := ioutil.ReadFile(configPath)
	if err != nil {
		fmt.Println("Unable to read configuration file: ", err)
		os.Exit(1)
	}
	err = yaml.Unmarshal(configFile, &Config)
	if err != nil {
		fmt.Println("Unable to parse yaml configuration: ", err)
		os.Exit(1)
	}
	// If URL wasnt provided, exit
	if Config.URL == "" {
		fmt.Println("Invalid configuration - URL: ", err)
		os.Exit(1)
	}
	// If Manifest wasnt provided, exit
	if Config.Manifest == "" {
		fmt.Println("Invalid configuration - Manifest: ", err)
		os.Exit(1)
	}
	// If CachePath wasn't provided, configure a default
	if Config.CachePath == "" {
		Config.CachePath = filepath.Join(os.Getenv("ProgramData"), "gorilla/cache")
	}
	// Set the verbosity
	if verbose == true && !Config.Verbose {
		Config.Verbose = true
	}
	// Set the debug and verbose
	if debug == true && !Config.Debug {
		Config.Debug = true
		Config.Verbose = true
	}

	// Set global variables
	URL = Config.URL
	Manifest = Config.Manifest
	Catalog = Config.Catalog
	CachePath = Config.CachePath
	Verbose = Config.Verbose
	Debug = Config.Debug
	AuthUser = Config.AuthUser
	AuthPass = Config.AuthPass
	TLSAuth = Config.TLSAuth
	TLSClientCert = Config.TLSClientCert
	TLSClientKey = Config.TLSClientKey
	TLSServerCert = Config.TLSServerCert

	// Add to GorillaReport
	report.Items["Manifest"] = Manifest
	report.Items["Catalog"] = Catalog

	return
}
