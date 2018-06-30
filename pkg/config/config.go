package config

import (
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Object to store our configuration
type Object struct {
	URL       string `yaml:"url"`
	Manifest  string `yaml:"manifest"`
	Catalog   string `yaml:"catalog"`
	CachePath string `yaml:"cachepath"`
	Verbose   bool   `yaml:"verbose,omitempty"`
}

func parseArguments() (string, bool) {
	// Get the command line args, error if config is missing.
	configArg := flag.String("config", "", "Path to configuration file in yaml format")
	verboseArg := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()
	if *configArg == "" {
		fmt.Println("Configuration file required!")
		flag.PrintDefaults()
		os.Exit(1)
	}

	return *configArg, *verboseArg
}

// Get returns the local configuration as a config.Object
func Get() Object {

	configPath, verbose := parseArguments()

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

	return configuration
}
