package config

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
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
)

// Object to store our configuration
type Object struct {
	URL       string `yaml:"url"`
	Manifest  string `yaml:"manifest"`
	Catalog   string `yaml:"catalog"`
	CachePath string `yaml:"cachepath"`
	Verbose   bool   `yaml:"verbose,omitempty"`
	Debug     bool   `yaml:"debug,omitempty"`
}

func parseArguments() (string, bool, bool) {
	// Get the command line args, error if config is missing.
	helpArg := flag.String("help", "", "Displays this help message")
	configArg := flag.String("config", filepath.Join(os.Getenv("ProgramData"), "gorilla/config.yaml"), "Path to configuration file in yaml format")
	verboseArg := flag.Bool("verbose", false, "Enable verbose output")
	debugArg := flag.Bool("debug", false, "Enable debug output")
	flag.Parse()
	if *helpArg != "" {
		flag.PrintDefaults()
		os.Exit(0)
	}

	return *configArg, *verboseArg, *debugArg
}

// Get retrieves and then stores the local configuration
func Get() {

	configPath, verbose, debug := parseArguments()

	// Get the config at configpath and return a config.Object
	configFile, err := ioutil.ReadFile(configPath)
	if err != nil {
		fmt.Println("Unable to read configuration file: ", err)
		os.Exit(1)
	}
	var configuration Object
	err = yaml.Unmarshal(configFile, &configuration)
	if err != nil {
		fmt.Println("Unable to parse yaml configuration: ", err)
		os.Exit(1)
	}
	// If URL wasnt provided, exit
	if configuration.URL == "" {
		fmt.Println("Invalid configuration - URL: ", err)
		os.Exit(1)
	}
	// If Manifest wasnt provided, exit
	if configuration.Manifest == "" {
		fmt.Println("Invalid configuration - Manifest: ", err)
		os.Exit(1)
	}
	// If CachePath wasn't provided, configure a default
	if configuration.CachePath == "" {
		configuration.CachePath = filepath.Join(os.Getenv("ProgramData"), "gorilla/cache")
	}
	// Set the verbosity
	if verbose == true && !configuration.Verbose {
		configuration.Verbose = true
	}
	// Set the debug and verbose
	if debug == true && !configuration.Debug {
		configuration.Debug = true
		configuration.Verbose = true
	}

	URL = configuration.URL
	Manifest = configuration.Manifest
	Catalog = configuration.Catalog
	CachePath = configuration.CachePath
	Verbose = configuration.Verbose
	Debug = configuration.Debug

	return
}
