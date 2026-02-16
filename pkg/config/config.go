package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v4"

	"github.com/1dustindavis/gorilla/pkg/report"
	"github.com/1dustindavis/gorilla/pkg/version"
)

var (
	// CachePath is a directory we will use for temporary storage
	cachePath string

	// Define flag defaults
	aboutArg          bool
	aboutDefault      = false
	configArg         string
	configDefault     = filepath.Join(os.Getenv("ProgramData"), "gorilla/config.yaml")
	debugArg          bool
	debugDefault      = false
	buildArg          bool
	buildDefault      = false
	importArg         string
	importDefault     = ""
	helpArg           bool
	helpDefault       = false
	verboseArg        bool
	verboseDefault    = false
	checkOnlyArg      bool
	checkOnlyDefault  = false
	versionArg        bool
	versionDefault    = false
	serviceArg        bool
	serviceDefault    = false
	serviceCmdArg     string
	serviceCmdDefault = ""
	serviceInstallArg bool
	serviceRemoveArg  bool
	serviceStartArg   bool
	serviceStopArg    bool
	serviceStatusArg  bool

	// Use a fake function so we can override when testing
	osExit = os.Exit
)

const usage = `
Gorilla - Munki-like Application Management for Windows
https://github.com/1dustindavis/gorilla

Usage: gorilla.exe [options]

Options:
-c, -config         path to configuration file in yaml format
-C, -checkonly	    enable check only mode
-b, -build          build catalog files from package-info files
-i, -import         create a package-info file from an installer package
-v, -verbose        enable verbose output
-d, -debug          enable debug output
-a, -about          displays the version number and other build info
-V, -version        display the version number
-s, -service        run Gorilla as a Windows service
-S, -servicecmd     send a command to a running Gorilla service (ListOptionalInstalls|InstallItem:itemName|RemoveItem:itemName|StreamOperationStatus:operationId)
-serviceinstall     install Gorilla as a Windows service
-serviceremove      remove Gorilla Windows service
-servicestart       start Gorilla Windows service
-servicestop        stop Gorilla Windows service
-servicestatus      show Gorilla Windows service status
-h, -help           display this help message

`

// Configuration stores all of the possible parameters a config file could contain
type Configuration struct {
	URL             string   `yaml:"url"`
	URLPackages     string   `yaml:"url_packages"`
	Manifest        string   `yaml:"manifest"`
	LocalManifests  []string `yaml:"local_manifests,omitempty"`
	Catalogs        []string `yaml:"catalogs"`
	AppDataPath     string   `yaml:"app_data_path"`
	Verbose         bool     `yaml:"verbose,omitempty"`
	Debug           bool     `yaml:"debug,omitempty"`
	CheckOnly       bool     `yaml:"checkonly,omitempty"`
	BuildArg        bool
	ImportArg       string
	RepoPath        string `yaml:"repo_path,omitempty"`
	AuthUser        string `yaml:"auth_user,omitempty"`
	AuthPass        string `yaml:"auth_pass,omitempty"`
	TLSAuth         bool   `yaml:"tls_auth,omitempty"`
	TLSClientCert   string `yaml:"tls_client_cert,omitempty"`
	TLSClientKey    string `yaml:"tls_client_key,omitempty"`
	TLSServerCert   string `yaml:"tls_server_cert,omitempty"`
	CachePath       string
	ServiceMode     bool `yaml:"service_mode,omitempty"`
	ServiceCommand  string
	ServiceInstall  bool
	ServiceRemove   bool
	ServiceStart    bool
	ServiceStop     bool
	ServiceStatus   bool
	ServiceName     string `yaml:"service_name,omitempty"`
	ServiceInterval string `yaml:"service_interval,omitempty"`
	ServicePipeName string `yaml:"service_pipe_name,omitempty"`
	ConfigPath      string
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
	// Build
	flag.BoolVar(&buildArg, "build", buildDefault, "")
	flag.BoolVar(&buildArg, "b", buildDefault, "")
	// Import
	flag.StringVar(&importArg, "import", importDefault, "")
	flag.StringVar(&importArg, "i", importDefault, "")
	// Checkonly
	flag.BoolVar(&checkOnlyArg, "checkonly", checkOnlyDefault, "")
	flag.BoolVar(&checkOnlyArg, "C", checkOnlyDefault, "")
	// Help
	flag.BoolVar(&helpArg, "help", helpDefault, "")
	flag.BoolVar(&helpArg, "h", helpDefault, "")
	// Verbose
	flag.BoolVar(&verboseArg, "verbose", verboseDefault, "")
	flag.BoolVar(&verboseArg, "v", verboseDefault, "")
	// Version
	flag.BoolVar(&versionArg, "version", versionDefault, "")
	flag.BoolVar(&versionArg, "V", versionDefault, "")
	// Service mode
	flag.BoolVar(&serviceArg, "service", serviceDefault, "")
	flag.BoolVar(&serviceArg, "s", serviceDefault, "")
	// Service command
	flag.StringVar(&serviceCmdArg, "servicecmd", serviceCmdDefault, "")
	flag.StringVar(&serviceCmdArg, "S", serviceCmdDefault, "")
	// Service install/remove/start/stop
	flag.BoolVar(&serviceInstallArg, "serviceinstall", false, "")
	flag.BoolVar(&serviceRemoveArg, "serviceremove", false, "")
	flag.BoolVar(&serviceStartArg, "servicestart", false, "")
	flag.BoolVar(&serviceStopArg, "servicestop", false, "")
	flag.BoolVar(&serviceStatusArg, "servicestatus", false, "")
}

func parseArguments() (string, bool, bool, bool, bool, string) {
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

	return configArg, verboseArg, debugArg, checkOnlyArg, buildArg, importArg
}

// Get retrieves and parses the config file and returns a Configuration struct and any errors
func Get() Configuration {
	var cfg Configuration

	// Parse any arguments that may have been passed
	configPath, verbose, debug, checkonly, build, importValue := parseArguments()

	// Read the config file
	configFile, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Println("Unable to read configuration file: ", err)
		osExit(1)
	}

	// Parse the config into a struct
	err = yaml.Unmarshal(configFile, &cfg)
	if err != nil {
		fmt.Println("Unable to parse yaml configuration: ", err)
		osExit(1)
	}

	serviceControlMode := serviceInstallArg || serviceRemoveArg || serviceStartArg || serviceStopArg || serviceStatusArg
	serviceClientMode := serviceCmdArg != ""

	// Normal run mode requires both manifest and URL.
	if !cfg.BuildArg && cfg.ImportArg == "" && !serviceControlMode && !serviceClientMode {
		if cfg.Manifest == "" {
			fmt.Println("Invalid configuration - Manifest: ", err)
			osExit(1)
		}

		if cfg.URL == "" {
			fmt.Println("Invalid configuration - URL: ", err)
			osExit(1)
		}
	}

	// If URLPackages wasn't provided, use the repo URL when available.
	if cfg.URLPackages == "" && cfg.URL != "" {
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

	if checkonly && !cfg.CheckOnly {
		cfg.CheckOnly = true
	}
	cfg.BuildArg = build
	cfg.ImportArg = importValue
	cfg.ConfigPath = configPath
	cfg.ServiceMode = serviceArg
	cfg.ServiceCommand = serviceCmdArg
	cfg.ServiceInstall = serviceInstallArg
	cfg.ServiceRemove = serviceRemoveArg
	cfg.ServiceStart = serviceStartArg
	cfg.ServiceStop = serviceStopArg
	cfg.ServiceStatus = serviceStatusArg

	// Set the cache path
	cfg.CachePath = filepath.Join(cfg.AppDataPath, "cache")
	serviceManifestPath := filepath.Join(cfg.AppDataPath, "service-manifest.yaml")
	var hasServiceManifest bool
	for _, localManifest := range cfg.LocalManifests {
		if localManifest == serviceManifestPath {
			hasServiceManifest = true
			break
		}
	}
	if !hasServiceManifest {
		cfg.LocalManifests = append(cfg.LocalManifests, serviceManifestPath)
	}

	// If RepoPath wasn't provided, default to current working directory.
	if cfg.RepoPath == "" {
		repoPath, wdErr := os.Getwd()
		if wdErr == nil {
			cfg.RepoPath = repoPath
		}
	} else {
		cfg.RepoPath = filepath.Clean(cfg.RepoPath)
	}

	// Add to GorillaReport
	report.Items["Manifest"] = cfg.Manifest
	report.Items["Catalog"] = cfg.Catalogs

	// Configure service defaults.
	if cfg.ServiceName == "" {
		cfg.ServiceName = "gorilla"
	}
	if cfg.ServiceInterval == "" {
		cfg.ServiceInterval = "1h"
	}
	if cfg.ServicePipeName == "" {
		cfg.ServicePipeName = "gorilla-service"
	}

	return cfg
}
