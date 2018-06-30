package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
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

	if resp.StatusCode <= 200 && resp.StatusCode >= 299 {
		return fmt.Errorf("%s : Download status code: %d", fileName, resp.StatusCode)
	}

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

type catalogItem struct {
	DisplayName           string   `yaml:"display_name"`
	InstallerItemLocation string   `yaml:"installer_item_location"`
	InstallerItemHash     string   `yaml:"installer_item_hash"`
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

type manifestObject struct {
	Name       string   `yaml:"name"`
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

func getManifests() []manifestObject {
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

		// Download the manifest
		manifestURL := config.URL + "manifests/" + currentManifest + ".yaml"
		err := downloadFile(config.CachePath, manifestURL)
		if err != nil {
			fmt.Println("Unable to retrieve manifest:", currentManifest, err)
			os.Exit(1)
		}

		// Get new manifest
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
			// manifests = append([]manifestObject{newManifest}, manifests...)
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
	Verbose   bool   `yaml:"verbose,omitempty"`
}

func getConfig(configPath string) configObject {
	// Get the config at configpath and return a configObject
	configFile, err := ioutil.ReadFile(configPath)
	if err != nil {
		fmt.Println("Unable to read configuration file: ", err)
		os.Exit(1)
	}
	var configuration configObject
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
	return configuration
}

func checkHash(file string, sha string) bool {
	f, err := os.Open(file)
	if err != nil {
		fmt.Printf("Unable to open file %s\n", err)
		return false
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		fmt.Printf("Unable to verify hash due to IO error: %s\n", err)
		return false
	}
	shaHash := hex.EncodeToString(h.Sum(nil))
	if shaHash != sha {
		fmt.Println("Downloaded file hash does not match config hash")
		return false
	}
	return true
}

func runCommand(action string, item catalogItem) {

	// Get all the path strings we will need
	tokens := strings.Split(item.InstallerItemLocation, "/")
	fileName := tokens[len(tokens)-1]
	relPath := strings.Join(tokens[:len(tokens)-1], "/")
	absPath := filepath.Join(config.CachePath, relPath)
	absFile := filepath.Join(absPath, fileName)
	fileExt := strings.ToLower(filepath.Ext(absFile))

	// If the file exists, check the hash
	var verified bool
	if _, err := os.Stat(absFile); err == nil {
		verified = checkHash(absFile, item.InstallerItemHash)
	}

	// If hash failed, download the installer
	if !verified {
		fmt.Printf("Downloading %s...\n", fileName)
		// Download the installer
		installerURL := config.URL + item.InstallerItemLocation
		err := downloadFile(absPath, installerURL)
		if err != nil {
			fmt.Println("Unable to retrieve package:", item.InstallerItemLocation, err)
			os.Exit(1)
		}
		verified = checkHash(absFile, item.InstallerItemHash)
	}

	// Return if hash verification fails
	if !verified {
		fmt.Println("Hash mismatch:", fileName)
		return
	}

	// Define the command and arguments based on the installer type
	var installCmd string
	var installArgs []string

	if fileExt == ".nupkg" {
		fmt.Println("Installing nupkg/choco:", fileName)
		installCmd = filepath.Join(os.Getenv("ProgramData"), "chocolatey/bin/choco.exe")
		installArgs = []string{action, absFile, "-y", "-r"}

	} else if fileExt == ".msi" {
		fmt.Println("Installing MSI for", fileName)
		installCmd = filepath.Join(os.Getenv("WINDIR"), "system32/", "msiexec.exe")
		installArgs = []string{"/I", absFile, "/quiet"}

	} else if fileExt == ".exe" {
		fmt.Println("EXE support not added yet:", fileName)
		return
	} else if fileExt == ".ps1" {
		fmt.Println("Powershell support not added yet:", fileName)
		return
	} else {
		fmt.Println("Unable to install", fileName)
		fmt.Println("Installer type unsupported:", fileExt)
		return
	}

	cmd := exec.Command(installCmd, installArgs...)
	cmdReader, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("command:", installCmd, installArgs)
		fmt.Fprintln(os.Stderr, "Error creating pipe to stdout", err)
		os.Exit(1)
	}

	scanner := bufio.NewScanner(cmdReader)
	if config.Verbose {
		fmt.Println("command:", installCmd, installArgs)
		go func() {
			for scanner.Scan() {
				fmt.Printf("Installer output | %s\n", scanner.Text())
			}
		}()
	}

	err = cmd.Start()
	if err != nil {
		fmt.Println("command:", installCmd, installArgs)
		fmt.Println(os.Stderr, "Error running command:", err)
		os.Exit(1)
	}

	err = cmd.Wait()
	if err != nil {
		fmt.Println("command:", installCmd, installArgs)
		fmt.Println(os.Stderr, "Installer error:", err)
		os.Exit(1)
	}

	return
}

var config configObject

func main() {
	// Get the command line args, error if blank.
	configArg := flag.String("config", "", "Path to configuration file in yaml format")
	verboseArg := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()
	if *configArg == "" {
		fmt.Println("Configuration file required!")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Get the actual configuration
	config = getConfig(*configArg)

	// Set the verbosity
	if *verboseArg == true && !config.Verbose {
		config.Verbose = true
	}

	// Download the catalog
	catalogURL := config.URL + "catalogs/" + config.Catalog + ".yaml"
	err := downloadFile(config.CachePath, catalogURL)
	if err != nil {
		fmt.Println("Unable to retrieve catalog:", config.Catalog, err)
		os.Exit(1)
	}
	
	// Parse the catalog
	catalog := getCatalog(config.CachePath, config.Catalog)

	// Get the manifests
	manifests := getManifests()

	// Compile all of the installs, uninstalls, and upgrades into arrays
	var installs, uninstalls, upgrades []string
	for _, manifest := range manifests {
		// Installs
		for _, item := range manifest.Installs {
			if item != "" {
				installs = append(installs, item)
			}
		}
		// Uninstalls
		for _, item := range manifest.Uninstalls {
			if item != "" {
				uninstalls = append(uninstalls, item)
			}
		}
		// Upgrades
		for _, item := range manifest.Upgrades {
			if item != "" {
				upgrades = append(upgrades, item)
			}
		}
	}

	// Iterate through the installs array, install dependencies, and then the item itself.
	for _, item := range installs {
		// Check for dependencies and install if found
		if len(catalog[item].Dependencies) > 0 {
			for _, dependency := range catalog[item].Dependencies {
				runCommand("install", catalog[dependency])
			}
		}
		// Install the item
		runCommand("install", catalog[item])

	}

}
