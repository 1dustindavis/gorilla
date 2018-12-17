package installer

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/download"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	"github.com/1dustindavis/gorilla/pkg/report"
	"github.com/1dustindavis/gorilla/pkg/status"
)

var (
	// Base command for each installer type
	commandNupkg = filepath.Join(os.Getenv("ProgramData"), "chocolatey/bin/choco.exe")
	commandMsi   = filepath.Join(os.Getenv("WINDIR"), "system32/", "msiexec.exe")
	commandPs1   = filepath.Join(os.Getenv("WINDIR"), "system32/", "WindowsPowershell", "v1.0", "powershell.exe")

	// This abstraction allows us to override when testing
	execCommand = exec.Command

	// Stores url where we will download an item
	installerURL   string
	uninstallerURL string
)

// runCommand executes a command and it's argurments in the CMD enviroment
func runCommand(command string, arguments []string) string {
	cmd := execCommand(command, arguments...)
	var cmdOutput bytes.Buffer
	cmdReader, err := cmd.StdoutPipe()
	cmd.Stdout = &cmdOutput
	if err != nil {
		gorillalog.Warn("command:", command, arguments)
		gorillalog.Error("Error creating pipe to stdout", err)
	}

	scanner := bufio.NewScanner(cmdReader)
	if config.Current.Verbose {
		gorillalog.Debug("command:", command, arguments)
		go func() {
			for scanner.Scan() {
				gorillalog.Debug("Command output | ", scanner.Text())
			}
		}()
	}

	err = cmd.Start()
	if err != nil {
		gorillalog.Warn("command:", command, arguments)
		gorillalog.Error("Error running command:", err)
	}

	err = cmd.Wait()
	if err != nil {
		gorillalog.Warn("command:", command, arguments)
		gorillalog.Error("Command error:", err)
	}
	return cmdOutput.String()
}

func installItem(item catalog.Item) string {

	// Determine the paths needed for download and install
	relPath, fileName := path.Split(item.InstallerItemLocation)
	absPath := filepath.Join(config.CachePath, relPath)
	absFile := filepath.Join(absPath, fileName)
	if config.Current.URLPackages != "" {
		installerURL = config.Current.URLPackages + item.InstallerItemLocation
	} else {
		installerURL = config.Current.URL + item.InstallerItemLocation
	}

	// Determine the install type and build the command
	var installCmd string
	var installArgs []string
	if item.InstallerType == "nupkg" {
		gorillalog.Info("Installing nupkg for", item.DisplayName)
		installCmd = commandNupkg
		installArgs = []string{"install", absFile, "-f", "-y", "-r"}
	} else if item.InstallerType == "msi" {
		gorillalog.Info("Installing msi for", item.DisplayName)
		installCmd = commandMsi
		installArgs = []string{"/i", absFile, "/qn", "/norestart"}
	} else if item.InstallerType == "exe" {
		gorillalog.Info("Installing exe for", item.DisplayName)
		installCmd = absFile
		installArgs = item.InstallerItemArguments
	} else if item.InstallerType == "ps1" {
		gorillalog.Info("Installing ps1 for", item.DisplayName)
		installCmd = commandPs1
		installArgs = []string{"-NoProfile", "-NoLogo", "-NonInteractive", "-WindowStyle", "Normal", "-ExecutionPolicy", "Bypass", "-File", absFile}

	} else {
		msg := fmt.Sprint("Unsupported installer type", item.InstallerType)
		gorillalog.Warn(msg)
		return msg
	}

	// Download the item if it is needed
	valid := download.IfNeeded(absFile, installerURL, item.InstallerItemHash)
	if !valid {
		msg := fmt.Sprint("Unable to download valid file: ", installerURL)
		gorillalog.Warn(msg)
		return msg
	}

	// Run the command
	installerOut := runCommand(installCmd, installArgs)

	// Add the item to InstalledItems in GorillaReport
	report.InstalledItems = append(report.InstalledItems, item)

	return installerOut
}

func uninstallItem(item catalog.Item) string {

	// Determine the paths needed for download and uinstall
	relPath, fileName := path.Split(item.UninstallerItemLocation)
	absPath := filepath.Join(config.CachePath, relPath)
	absFile := filepath.Join(absPath, fileName)
	if config.Current.URLPackages != "" {
		uninstallerURL = config.Current.URLPackages + item.UninstallerItemLocation
	} else {
		uninstallerURL = config.Current.URL + item.UninstallerItemLocation
	}

	// Determine the uninstall type and build the command
	var uninstallCmd string
	var uninstallArgs []string
	if item.UninstallerType == "nupkg" {
		gorillalog.Info("Installing nupkg for", item.DisplayName)
		uninstallCmd = commandNupkg
		uninstallArgs = []string{"uninstall", absFile, "-f", "-y", "-r"}
	} else if item.UninstallerType == "msi" {
		gorillalog.Info("Installing msi for", item.DisplayName)
		uninstallCmd = commandMsi
		uninstallArgs = []string{"/x", absFile, "/qn", "/norestart"}
	} else if item.UninstallerType == "exe" {
		gorillalog.Info("Installing exe for", item.DisplayName)
		uninstallCmd = absFile
		uninstallArgs = item.UninstallerItemArguments
	} else if item.UninstallerType == "ps1" {
		gorillalog.Info("Installing ps1 for", item.DisplayName)
		uninstallCmd = commandPs1
		uninstallArgs = []string{"-NoProfile", "-NoLogo", "-NonInteractive", "-WindowStyle", "Normal", "-ExecutionPolicy", "Bypass", "-File", absFile}

	} else {
		msg := fmt.Sprint("Unsupported uninstaller type", item.UninstallerType)
		gorillalog.Warn(msg)
		return msg
	}

	// Download the item if it is needed
	valid := download.IfNeeded(absFile, uninstallerURL, item.UninstallerItemHash)
	if !valid {
		msg := fmt.Sprint("Unable to download valid file: ", installerURL)
		gorillalog.Warn(msg)
		return msg
	}

	// Run the command
	uninstallerOut := runCommand(uninstallCmd, uninstallArgs)

	// Add the item to InstalledItems in GorillaReport
	report.UninstalledItems = append(report.UninstalledItems, item)

	return uninstallerOut
}

// Install determines if action needs to be taken on a item and then
// calls the appropriate function to install or uninstall
func Install(item catalog.Item, installerType string) string {
	// Check the status and determine if any action is needed for this item
	actionNeeded, err := status.CheckStatus(item, installerType)
	if err != nil {
		msg := fmt.Sprint("Unable to check status: ", err)
		gorillalog.Warn(msg)
		return msg
	}

	// If no action is needed, return
	if !actionNeeded {
		return "Item not needed"
	}

	// Install or uninstall the item
	if installerType == "install" || installerType == "update" {
		installItem(item)
	} else if installerType == "uninstall" {
		uninstallItem(item)
	} else {
		gorillalog.Warn("Unsupported item type", item.DisplayName, installerType)
		return "Unsupported item type"

	}

	return ""
}
