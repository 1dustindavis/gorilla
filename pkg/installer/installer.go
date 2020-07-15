package installer

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/1dustindavis/gorilla/pkg/catalog"
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

	// These abstractions allows us to override when testing
	execCommand       = exec.Command
	statusCheckStatus = status.CheckStatus

	// Stores url where we will download an item
	installerURL   string
	uninstallerURL string
)

// runCommand executes a command and it's argurments in the CMD environment
func runCommand(command string, arguments []string) string {
	cmd := execCommand(command, arguments...)
	var cmdOutput string
	cmdReader, err := cmd.StdoutPipe()
	if err != nil {
		gorillalog.Warn("command:", command, arguments)
		gorillalog.Warn("Error creating pipe to stdout", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	scanner := bufio.NewScanner(cmdReader)
	gorillalog.Debug("command:", command, arguments)
	go func() {
		gorillalog.Debug("Command Output:")
		gorillalog.Debug("--------------------")
		for scanner.Scan() {
			gorillalog.Debug(scanner.Text())
			cmdOutput = scanner.Text()
		}
		gorillalog.Debug("--------------------")
		wg.Done()
	}()

	err = cmd.Start()
	if err != nil {
		gorillalog.Warn("command:", command, arguments)
		gorillalog.Warn("Error running command:", err)
	}

	wg.Wait()
	err = cmd.Wait()
	if err != nil {
		gorillalog.Warn("command:", command, arguments)
		gorillalog.Warn("Command error:", err)
	}

	return cmdOutput
}

// Get a Nupkg's id using `choco list`
func getNupkgID(nupkgDir, versionArg string) string {

	// Compile the arguments needed to get the id
	command := commandNupkg
	arguments := []string{"list", versionArg, "--id-only", "-r", "-s", nupkgDir}

	// Run the command and trim the output
	cmdOut := runCommand(command, arguments)
	nupkgID := strings.TrimSpace(cmdOut)

	// The final output should just be the nupkg id
	return nupkgID
}

func installItem(item catalog.Item, itemURL, cachePath string) string {

	// Determine the paths needed for download and install
	relPath, fileName := path.Split(item.Installer.Location)
	absPath := filepath.Join(cachePath, relPath)
	absFile := filepath.Join(absPath, fileName)

	// Download the item if it is needed
	valid := download.IfNeeded(absFile, itemURL, item.Installer.Hash)
	if !valid {
		msg := fmt.Sprint("Unable to download valid file: ", itemURL)
		gorillalog.Warn(msg)
		return msg
	}

	// Determine the install type and command to pass
	var installCmd string
	var installArgs []string
	if item.Installer.Type == "nupkg" {
		// choco wants the "id" and parent dir when we install, so we need to determine both
		gorillalog.Info("Determining nupkg id for", item.DisplayName)
		nupkgDir := filepath.Dir(absFile)

		// Since choco recommends the source is a directory,
		// we need to pass a version to filter unexpected nupkgs (if we have a version)
		var versionArg string
		var nupkgID string
		if item.Version != "" {
			versionArg = fmt.Sprintf("--version=%s", item.Version)
			nupkgID = getNupkgID(nupkgDir, versionArg)
		}

		// Now pass the id along with the parent directory
		gorillalog.Info("Installing nupkg for", item.DisplayName)
		installCmd = commandNupkg
		if nupkgID != "" && versionArg != "" {
			// Only use this form if we have an ID and version number
			installArgs = []string{"install", nupkgID, "-s", nupkgDir, versionArg, "-f", "-y", "-r"}
		} else {
			// If we dont have an id and version, fallback to the method choco doesn't recommend (but works)
			installArgs = []string{"install", absFile, "-f", "-y", "-r"}
		}

	} else if item.Installer.Type == "msi" {
		gorillalog.Info("Installing msi for", item.DisplayName)
		installCmd = commandMsi
		installArgs = []string{"/i", absFile, "/qn", "/norestart"}
		installArgs = append(installArgs, item.Installer.Arguments...)

	} else if item.Installer.Type == "exe" {
		gorillalog.Info("Installing exe for", item.DisplayName)
		installCmd = absFile
		installArgs = item.Installer.Arguments

	} else if item.Installer.Type == "ps1" {
		gorillalog.Info("Installing ps1 for", item.DisplayName)
		installCmd = commandPs1
		installArgs = []string{"-NoProfile", "-NoLogo", "-NonInteractive", "-WindowStyle", "Normal", "-ExecutionPolicy", "Bypass", "-File", absFile}

	} else {
		msg := fmt.Sprint("Unsupported installer type", item.Installer.Type)
		gorillalog.Warn(msg)
		return msg
	}

	// Run the command
	installerOut := runCommand(installCmd, installArgs)

	// Add the item to InstalledItems in GorillaReport
	report.InstalledItems = append(report.InstalledItems, item)

	return installerOut
}

func uninstallItem(item catalog.Item, itemURL, cachePath string) string {

	// Determine the paths needed for download and uinstall
	relPath, fileName := path.Split(item.Uninstaller.Location)
	absPath := filepath.Join(cachePath, relPath)
	absFile := filepath.Join(absPath, fileName)

	// Download the item if it is needed
	valid := download.IfNeeded(absFile, itemURL, item.Uninstaller.Hash)
	if !valid {
		msg := fmt.Sprint("Unable to download valid file: ", itemURL)
		gorillalog.Warn(msg)
		return msg
	}

	// Determine the uninstall type and build the command
	var uninstallCmd string
	var uninstallArgs []string

	if item.Uninstaller.Type == "nupkg" {
		// choco wants the "id" and parent dir when we uninstall, so we need to determine both
		gorillalog.Info("Determining nupkg id for", item.DisplayName)
		nupkgDir := filepath.Dir(absFile)

		// Since choco recommends the source is a directory,
		// we need to pass a version to filter unexpected nupkgs (if we have a version)
		var versionArg string
		var nupkgID string
		if item.Version != "" {
			versionArg = fmt.Sprintf("--version=%s", item.Version)
			nupkgID = getNupkgID(nupkgDir, versionArg)
		}

		// Now pass the id along with the parent directory
		gorillalog.Info("Uninstalling nupkg for", item.DisplayName)
		uninstallCmd = commandNupkg
		if nupkgID != "" && versionArg != "" {
			// Only use this form if we have an ID and version number
			uninstallArgs = []string{"uninstall", nupkgID, "-s", nupkgDir, versionArg, "-f", "-y", "-r"}
		} else {
			// If we dont have an id and version, fallback to the method choco doesn't recommend (but works)
			uninstallArgs = []string{"uninstall", absFile, "-f", "-y", "-r"}
		}

	} else if item.Uninstaller.Type == "msi" {
		gorillalog.Info("Uninstalling msi for", item.DisplayName)
		uninstallCmd = commandMsi
		uninstallArgs = []string{"/x", absFile, "/qn", "/norestart"}

	} else if item.Uninstaller.Type == "exe" {
		gorillalog.Info("Uninstalling exe for", item.DisplayName)
		uninstallCmd = absFile
		uninstallArgs = item.Uninstaller.Arguments

	} else if item.Uninstaller.Type == "ps1" {
		gorillalog.Info("Uninstalling ps1 for", item.DisplayName)
		uninstallCmd = commandPs1
		uninstallArgs = []string{"-NoProfile", "-NoLogo", "-NonInteractive", "-WindowStyle", "Normal", "-ExecutionPolicy", "Bypass", "-File", absFile}

	} else {
		msg := fmt.Sprint("Unsupported uninstaller type", item.Uninstaller.Type)
		gorillalog.Warn(msg)
		return msg
	}

	// Run the command
	uninstallerOut := runCommand(uninstallCmd, uninstallArgs)

	// Add the item to InstalledItems in GorillaReport
	report.UninstalledItems = append(report.UninstalledItems, item)

	return uninstallerOut
}

var (
	// By putting the functions in a variable, we can override later in tests
	installItemFunc   = installItem
	uninstallItemFunc = uninstallItem
)

// Install determines if action needs to be taken on a item and then
// calls the appropriate function to install or uninstall
func Install(item catalog.Item, installerType, urlPackages, cachePath string, checkOnly bool) string {
	// Check the status and determine if any action is needed for this item
	actionNeeded, err := statusCheckStatus(item, installerType, cachePath)
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
		// Check if checkonly mode is enabled
		if checkOnly {
			report.InstalledItems = append(report.InstalledItems, item)
			gorillalog.Info("[CHECK ONLY] Skipping actions for", item.DisplayName)
			// Check only mode doesn't perform any action, return
			return "Check only enabled"
		} else {
			// Compile the item's URL
			itemURL := urlPackages + item.Installer.Location
			// Run the installer
			installItemFunc(item, itemURL, cachePath)
		}
	} else if installerType == "uninstall" {
		if checkOnly {
			report.InstalledItems = append(report.InstalledItems, item)
			gorillalog.Info("[CHECK ONLY] Skipping actions for", item.DisplayName)
			// Check only mode doesn't perform any action, return
			return "Check only enabled"
		} else {
			// Compile the item's URL
			itemURL := urlPackages + item.Uninstaller.Location
			// Run the installer
			uninstallItemFunc(item, itemURL, cachePath)
		}
	} else {
		gorillalog.Warn("Unsupported item type", item.DisplayName, installerType)
		return "Unsupported item type"

	}

	return ""
}
