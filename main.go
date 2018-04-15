package main

import (
	"bufio"
	"encoding/json"
    "fmt"
	"io"
	"io/ioutil"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "flag"
    "strings"
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
    resp, err := http.Get(url)
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
	Version string
	Installed bool
}


func getStatus() error {
	// iterate through apps and get current status
	return nil
}


func downloadManifest(cachePath string, url string, manifest string) {
	// Download the manifest
	err := downloadFile(cachePath, (url + manifest + ".json"))
    if err != nil {
        fmt.Println("Unable to retrieve manifest: ", manifest, err)
		os.Exit(1)
    }
    return
}

type manifestObject struct {
  Name string `json:"name"`
  Includes []string `json:"included_manifests"`
  Installs []string `json:"managed_installs"`
  Uninstalls []string `json:"managed_uninstalls"`
  Upgrades []string `json:"managed_upgrades"`
}


func getManifest(cachePath string, manifestName string) manifestObject {
	// Get the json file and look for included_manifests
	jsonPath := filepath.Join(cachePath, manifestName) + ".json"
	jsonFile, err := ioutil.ReadFile(jsonPath)
    var manifest manifestObject
	err = json.Unmarshal(jsonFile, &manifest)
	if err != nil {
		fmt.Println("Unable to parse json manifest: ", jsonPath, err)
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
	URL string
	Manifest string
	CachePath string
}


func getConfig(configpath string) configObject {
	// Get the config at configpath and return a configObject
	configfile, err := ioutil.ReadFile(configpath)
	if err != nil {
		fmt.Println("Unable to read configuration file: ", err)
		os.Exit(1)
	}
	var config configObject
	err = json.Unmarshal(configfile, &config)
 	if err != nil {
	    fmt.Println("Unable to parse json configuration: ", err)
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

func chocoInstall(action string, item string) {
	chocoCmd := filepath.Join(os.Getenv("ProgramData"), "chocolatey/bin/choco.exe")
	chocoArgs := []string{action, item, "-y"}

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
	configArg := flag.String("config", "", "Path to configuration file in json format")
	flag.Parse()
	if *configArg == "" {
		fmt.Println("Configuration file required!")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Get the actual configuration
	config := getConfig(*configArg)

	// Get the manifests
	manifests := getManifests(config)

	// Compile all of the installs, uninstalls, and upgrades
	var actions map[string]string
	actions = make(map[string]string)
	for _, manifest := range manifests {
		for _, item := range manifest.Installs {
			if item != "" {
				actions[item] = "install"
			}
		}
		for _, item := range manifest.Uninstalls {
			if item != "" {
				actions[item] = "uninstall"
			}
		}
		for _, item := range manifest.Upgrades {
			if item != "" {
				actions[item] = "upgrade"
			}
		}
	}
	// fmt.Printf("\n")
	// fmt.Printf("%-10s %-10s\n", "Action", "Item")
	// fmt.Printf("%-10s %-10s\n", "----------", "----------")
	// for item, action := range actions {
	// 	fmt.Printf("%-10s %-10s\n", action, item)
	// }

	for item, action := range actions {
		chocoInstall(action, item)
	}

}
