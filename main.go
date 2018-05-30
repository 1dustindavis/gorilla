package main

import (
	"bufio"
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func downloadFile(file string, url string) error {
	// Get the absolute file path
	tokens := strings.Split(url, "/")
	fileName := tokens[len(tokens)-1]
	absPath := filepath.Join(file, fileName)

	// Create the dir and file
	err := os.MkdirAll(filepath.Clean(file), 0755)
	out, err := os.Create(filepath.Clean(absPath))
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	client := &http.Client{
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 10 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	resp, err := client.Get(url)

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

// Object to store application status
type statusObject struct {
	Application string
	Version     string
	Installed   bool
}

func getCurrentStatus() error {
	// iterate through apps and get current status
	return nil
}

func downloadCatalog(cachePath string, url string, catalog string) {
	err := downloadFile(cachePath, (url + "catalogs/" + catalog + ".yaml"))
	if err != nil {
		fmt.Println("Unable to retrieve catalog:", catalog, err)
		os.Exit(1)
	}
	return
}

type catalogItem struct {
	DisplayName           string   `yaml:"display_name"`
	InstallerItemLocation string   `yaml:"installer_item_location"`
	Version               string   `yaml:"version"`
	Dependencies          []string `yaml:"dependencies"`
}

func getCatalog(cachePath string, catalogName string) map[string]catalogItem {
	yamlPath := filepath.Join(cachePath, catalogName) + ".yaml"
	yamlFile, err := ioutil.ReadFile(yamlPath)
	var catalog map[string]catalogItem
	err = yaml.Unmarshal(yamlFile, &catalog)
	if err != nil {
		fmt.Println("Unable to parse yaml catalog:", yamlPath, err)
	}
	return catalog
}

func downloadManifest(cachePath string, url string, manifest string) {
	// Download the manifest
	err := downloadFile(cachePath, (url + "manifests/" + manifest + ".yaml"))
	if err != nil {
		fmt.Println("Unable to retrieve manifest:", manifest, err)
		os.Exit(1)
	}
	return
}

type manifestObject struct {
	Includes   []string `yaml:"included_manifests"`
	Installs   []string `yaml:"managed_installs"`
	Uninstalls []string `yaml:"managed_uninstalls"`
	Upgrades   []string `yaml:"managed_upgrades"`
}

func getManifest(cachePath string, manifestName string) manifestObject {
	// Get the yaml file and look for included_manifests
	yamlPath := filepath.Join(cachePath, manifestName) + ".yaml"
	yamlFile, err := ioutil.ReadFile(yamlPath)
	var manifest manifestObject
	err = yaml.Unmarshal(yamlFile, &manifest)
	if err != nil {
		fmt.Println("Unable to parse yaml manifest:", yamlPath, err)
	}
	return manifest
}

func getManifests(config configObject) []manifestObject {
	// Create a slice of all manifest objects
	var manifests []manifestObject
	// Create a slice with the names of all manifests
	// This is so we can track them before we get the data
	var manifestsList []string

	// Setup interation tracking for manifests
	var manifestsTotal = len(manifestsList)
	var manifestsProcessed = 0
	var manifestsRemaining = 1

	// Add the top level manifest to the list
	manifestsList = append(manifestsList, config.Manifest)

	for manifestsRemaining > 0 {
		currentManifest := manifestsList[manifestsProcessed]
		// Add the current manifest to our working list
		workingList := []string{currentManifest}
		// Get new manifest
		downloadManifest(config.CachePath, config.URL, currentManifest)
		newManifest := getManifest(config.CachePath, currentManifest)

		// Add any includes to our working list
		for _, item := range newManifest.Includes {
			workingList = append(workingList, item)
		}

		// Get workingList unique items, and add to the real list
		for _, item := range workingList {
			// Check if unique in manifestsList
			var uniqueInList = true
			for i := range manifestsList {
				if manifestsList[i] == item {
					uniqueInList = false
				}
			}
			// Update manifestsList if it is unique
			if uniqueInList {
				manifestsList = append(manifestsList, item)
			}
		}

		// Check if this is unique in manifests
		var uniqueInManifests = true
		for i := range manifests {
			if manifests[i].Name == newManifest.Name {
				uniqueInManifests = false
			}
		}
		// Update manifests
		if uniqueInManifests {
			manifests = append(manifests, newManifest)
		}

		// Increment counters
		manifestsTotal = len(manifestsList)
		manifestsProcessed++
		manifestsRemaining = manifestsTotal - manifestsProcessed
	}
	return manifests
}

// Object to store our configuration
type configObject struct {
	URL       string `yaml:"url"`
	Manifest  string `yaml:"manifest"`
	Catalog   string `yaml:"catalog"`
	CachePath string `yaml:"cachepath"`
}

func getConfig(configpath string) configObject {
	// Get the config at configpath and return a configObject
	configfile, err := ioutil.ReadFile(configpath)
	if err != nil {
		fmt.Println("Unable to read configuration file: ", err)
		os.Exit(1)
	}
	var config configObject
	err = yaml.Unmarshal(configfile, &config)
	if err != nil {
		fmt.Println("Unable to parse yaml configuration: ", err)
		os.Exit(1)
	}
	// If URL wasnt provided, exit
	if config.URL == "" {
		fmt.Println("Invalid configuration - URL: ", err)
		os.Exit(1)
	}
	// If Manifest wasnt provided, exit
	if config.Manifest == "" {
		fmt.Println("Invalid configuration - Manifest: ", err)
		os.Exit(1)
	}
	// If CachePath wasn't provided, configure a default
	if config.CachePath == "" {
		config.CachePath = filepath.Join(os.Getenv("ProgramData"), "gorilla/cache")
	}
	return config
}

func downloadPackage(relPath string, url string, packageLocation string) {
	// Download the manifest
	err := downloadFile(relPath, (url + packageLocation))
	if err != nil {
		fmt.Println("Unable to retrieve package:", packageLocation, err)
		os.Exit(1)
	}
	return
}

func chocoCommand(action string, item catalogItem, config configObject) {

	tokens := strings.Split(item.InstallerItemLocation, "/")
	fileName := tokens[len(tokens)-1]
	relPath := strings.Join(tokens[:len(tokens)-1], "/")
	absPath := filepath.Join(config.CachePath, relPath)
	absFile := filepath.Join(absPath, fileName)

	downloadPackage(absPath, config.URL, item.InstallerItemLocation)

	chocoCmd := filepath.Join(os.Getenv("ProgramData"), "chocolatey/bin/choco.exe")
	chocoArgs := []string{action, absFile, "-y", "-r"}

	fmt.Println("command:", chocoCmd, chocoArgs)
	cmd := exec.Command(chocoCmd, chocoArgs...)
	cmdReader, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("command:", chocoCmd, chocoArgs)
		fmt.Fprintln(os.Stderr, "Error creating StdoutPipe for choco", err)
		os.Exit(1)
	}

	scanner := bufio.NewScanner(cmdReader)
	go func() {
		for scanner.Scan() {
			fmt.Printf("choco output | %s\n", scanner.Text())
		}
	}()

	err = cmd.Start()
	if err != nil {
		fmt.Println("command:", chocoCmd, chocoArgs)
		fmt.Println(os.Stderr, "Error starting Cmd", err)
		os.Exit(1)
	}

	err = cmd.Wait()
	if err != nil {
		fmt.Println("command:", chocoCmd, chocoArgs)
		fmt.Println(os.Stderr, "Choco error", err)
		os.Exit(1)
	}

	return
}

func main() {
	// Get config file from command args, error if blank.
	configArg := flag.String("config", "", "Path to configuration file in yaml format")
	flag.Parse()
	if *configArg == "" {
		fmt.Println("Configuration file required!")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Get the actual configuration
	config := getConfig(*configArg)

	// Download and parse the catalog
	downloadCatalog(config.CachePath, config.URL, config.Catalog)
	catalog := getCatalog(config.CachePath, config.Catalog)

	// Get the manifests
	manifests := getManifests(config)

	// Compile all of the installs, uninstalls, and upgrades into arrays
	var installs, uninstalls, upgrades []string
	for _, manifest := range manifests {
		// Installs
		for _, item := range manifest.Installs {
			if item != "" {
				installs = append([]string{item}, installs...)
			}
		}
		// Uninstalls
		for _, item := range manifest.Uninstalls {
			if item != "" {
				uninstalls = append([]string{item}, uninstalls...)
			}
		}
		// Upgrades
		for _, item := range manifest.Upgrades {
			if item != "" {
				upgrades = append([]string{item}, upgrades...)
			}
		}
	}

	// Iterate through the installs array, install dependencies, and then the item itself.
	for _, item := range installs {
		// Check if the installer is available
		if catalog[item].InstallerItemLocation == "" {
			fmt.Println("installer_item_location missing for item:", item)
			continue
		}
		// Check for dependencies, install if found
		if len(catalog[item].Dependencies) > 0 {
			for _, dependency := range catalog[item].Dependencies {
				chocoCommand("install", catalog[dependency], config)
			}
		}
		// Install the item
		chocoCommand("install", catalog[item], config)
	}

}
