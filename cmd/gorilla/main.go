package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/download"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	"github.com/1dustindavis/gorilla/pkg/manifest"
	"github.com/1dustindavis/gorilla/pkg/process"
	"github.com/1dustindavis/gorilla/pkg/report"
)

func main() {

	// Get our configuration
	cfg := config.Get()

	// Confirm we are running as an administrator before continuing
	adminCheck()

	// If needed, create the cache directory
	err := os.MkdirAll(filepath.Clean(cfg.CachePath), 0755)
	if err != nil {
		fmt.Println("Unable to create cache directory: ", err)
		os.Exit(1)
	}

	// Create a new logger object
	gorillalog.NewLog(cfg)

	// Start creating GorillaReport
	report.Start()

	// Set the configuration that `download` will use
	download.SetConfig(cfg)

	// Get the manifests
	gorillalog.Info("Retrieving manifest:", cfg.Manifest)
	manifests, newCatalogs := manifest.Get(cfg)

	// If we have newCatalogs, add them to the configuration
	if newCatalogs != nil {
		cfg.Catalogs = append(cfg.Catalogs, newCatalogs...)
	}

	// Get the catalogs
	gorillalog.Info("Retrieving catalog:", cfg.Catalogs)
	catalogs := catalog.Get(cfg)

	// Process the manifests into install type groups
	gorillalog.Info("Processing manifest...")
	installs, uninstalls, updates := process.Manifests(manifests, catalogs)

	// Prepare and install
	gorillalog.Info("Processing managed installs...")
	process.Installs(installs, catalogs, cfg.URLPackages, cfg.CachePath, cfg.CheckOnly)

	// Prepare and uninstall
	gorillalog.Info("Processing managed uninstalls...")
	process.Uninstalls(uninstalls, catalogs, cfg.URLPackages, cfg.CachePath, cfg.CheckOnly)

	// Prepare and update
	gorillalog.Info("Processing managed updates...")
	process.Updates(updates, catalogs, cfg.URLPackages, cfg.CachePath, cfg.CheckOnly)

	// Save GorillaReport to disk
	gorillalog.Info("Saving GorillaReport.json...")
	report.End()

	// Run CleanUp to delete old cached items and empty directories
	gorillalog.Info("Cleaning up the cache...")
	process.CleanUp(cfg.CachePath)

	gorillalog.Info("Done!")
}

func adminCheck() {

	// Skip the check if this is test
	if flag.Lookup("test.v") != nil {
		return
	}

	// Compile the PowerShell command used to determine if the current user is an administrator
	currentUser := "(New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent()))"
	adminRole := "([Security.Principal.WindowsBuiltInRole]::Administrator)"
	checkCmd := currentUser + ".IsInRole" + adminRole

	// Execute the command with Powershell and capture the output
	cmdOutput, err := exec.Command("powershell.exe", "-Command", checkCmd).CombinedOutput()
	if err != nil {
		fmt.Println("Unable to determine current permissions via Powershell: ", err)
		fmt.Println("Gorilla requires admnisistrative access. Please run as an administrator.")
		os.Exit(1)
	}

	// Convert the output to a lowercase string
	strOutput := strings.ToLower(string(cmdOutput))

	// If the output contains the word "true", we are running as an administrator
	if strings.Contains(strOutput, "true") {
		return
	}

	// The user does not have the `Administrator` role
	fmt.Println("Gorilla requires admnisistrative access. Please run as an administrator.")
	os.Exit(1)
}
