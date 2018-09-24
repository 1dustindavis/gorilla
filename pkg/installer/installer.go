package installer

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/download"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	"github.com/1dustindavis/gorilla/pkg/report"
	"github.com/1dustindavis/gorilla/pkg/status"
)

// runCommand executes a command and it's argurments in the CMD enviroment
func runCommand(command string, arguments []string) {
	cmd := exec.Command(command, arguments...)
	cmdReader, err := cmd.StdoutPipe()
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
	return
}

// Install runs the installer
func Install(item catalog.Item) {

	// Check the items current status
	install, err := status.CheckStatus(item, "install")
	if err != nil {
		gorillalog.Warn("Unable to check status of ", item.DisplayName)
		return
	}

	if !install {
		return
	}

	// Get all the path strings we will need
	tokens := strings.Split(item.InstallerItemLocation, "/")
	fileName := tokens[len(tokens)-1]
	relPath := strings.Join(tokens[:len(tokens)-1], "/")
	absPath := filepath.Join(config.CachePath, relPath)
	absFile := filepath.Join(absPath, fileName)
	fileExt := strings.ToLower(filepath.Ext(absFile))

	// Fail if we dont have a hash
	if item.InstallerItemHash == "" {
		gorillalog.Warn("Installer hash missing for item:", item.DisplayName)
		return
	}
	// If the file exists, check the hash
	var verified bool
	if _, err := os.Stat(absFile); err == nil {
		verified = download.Verify(absFile, item.InstallerItemHash)
	}

	// If hash failed, download the installer
	if !verified {
		gorillalog.Info("Downloading", item.DisplayName)
		// Download the installer
		installerURL := config.Current.URL + item.InstallerItemLocation
		err := download.File(absPath, installerURL)
		if err != nil {
			gorillalog.Warn("Unable to retrieve package:", item.InstallerItemLocation, err)
			return
		}
		verified = download.Verify(absFile, item.InstallerItemHash)
	}

	// Return if hash verification fails
	if !verified {
		gorillalog.Warn("Hash mismatch:", item.DisplayName)
		return
	}

	// Define the command and arguments based on the installer type
	var installCmd string
	var installArgs []string

	if fileExt == ".nupkg" {
		gorillalog.Info("Installing nupkg:", fileName)
		installCmd = filepath.Join(os.Getenv("ProgramData"), "chocolatey/bin/choco.exe")
		installArgs = []string{"install", absFile, "-y", "-r"}

	} else if fileExt == ".msi" {
		gorillalog.Info("Installing MSI installer:", fileName)
		installCmd = filepath.Join(os.Getenv("WINDIR"), "system32/", "msiexec.exe")
		installArgs = []string{"/i", absFile, "/qn", "/norestart"}

	} else if fileExt == ".exe" {
		gorillalog.Info("Installing exe installer:", fileName)
		installCmd = absFile
		installArgs = item.InstallerItemArguments

	} else if fileExt == ".ps1" {
		gorillalog.Info("Installing Powershell script:", fileName)
		installCmd = filepath.Join(os.Getenv("WINDIR"), "system32/", "WindowsPowershell", "v1.0", "powershell.exe")
		installArgs = []string{"-NoProfile", "-NoLogo", "-NonInteractive", "-WindowStyle", "Normal", "-ExecutionPolicy", "Bypass", "-File", absFile}

	} else {
		gorillalog.Warn("Unable to install", fileName)
		gorillalog.Warn("Installer type unsupported:", fileExt)
		return
	}

	// Add the item to InstalledItems in GorillaReport
	report.InstalledItems = append(report.InstalledItems, item)

	// Run the command and arguments
	runCommand(installCmd, installArgs)

	return
}

// Uninstall runs the uninstaller
func Uninstall(item catalog.Item) {

	// Check the items current status
	install, err := status.CheckStatus(item, "uninstall")
	if err != nil {
		gorillalog.Warn("Unable to check status of ", item.DisplayName)
		return
	}

	if install {
		return
	}

	// Get all the path strings we will need
	tokens := strings.Split(item.InstallerItemLocation, "/")
	fileName := tokens[len(tokens)-1]
	relPath := strings.Join(tokens[:len(tokens)-1], "/")
	absPath := filepath.Join(config.CachePath, relPath)
	absFile := filepath.Join(absPath, fileName)

	// Fail if we dont have a hash
	if item.InstallerItemHash == "" {
		gorillalog.Warn("Installer hash missing for item:", item.DisplayName)
		return
	}

	// If the file exists, check the hash
	var verified bool
	if _, err := os.Stat(absFile); err == nil {
		verified = download.Verify(absFile, item.InstallerItemHash)
	}

	// If hash failed, download the installer
	if !verified {
		gorillalog.Info("Downloading", item.DisplayName)
		// Download the installer
		installerURL := config.Current.URL + item.InstallerItemLocation
		err := download.File(absPath, installerURL)
		if err != nil {
			gorillalog.Warn("Unable to retrieve package:", item.InstallerItemLocation, err)
			return
		}
		verified = download.Verify(absFile, item.InstallerItemHash)
	}

	// Return if hash verification fails
	if !verified {
		gorillalog.Warn("Hash mismatch:", item.DisplayName)
		return
	}

	// Define the command and arguments based on the installer type
	var uninstallCmd string
	var uninstallArgs []string

	if item.UninstallMethod == "choco" {
		gorillalog.Info("Uninstalling nupkg:", item.DisplayName)
		uninstallCmd = filepath.Join(os.Getenv("ProgramData"), "chocolatey/bin/choco.exe")
		uninstallArgs = []string{"uninstall", absFile, "-y", "-r"}

	} else if item.UninstallMethod == "msi" {
		gorillalog.Info("Unnstalling MSI", item.DisplayName)
		uninstallCmd = filepath.Join(os.Getenv("WINDIR"), "system32/", "msiexec.exe")
		uninstallArgs = []string{"/x", absFile, "/qn", "/norestart"}
	} else {
		gorillalog.Warn("Unable to uninstall", item.DisplayName)
		gorillalog.Warn("Installer type unsupported:", item.UninstallMethod)
		return
	}

	// Add the item to UninstalledItems in GorillaReport
	report.UninstalledItems = append(report.UninstalledItems, item)

	// Run the command and arguments
	runCommand(uninstallCmd, uninstallArgs)

	return
}

// Update runs the installer if the item is already installed, but not up-to-date
func Update(item catalog.Item) {

	// Check the items current status
	install, err := status.CheckStatus(item, "update")
	if err != nil {
		gorillalog.Warn("Unable to check status of ", item.DisplayName)
		return
	}

	if !install {
		return
	}

	// Get all the path strings we will need
	tokens := strings.Split(item.InstallerItemLocation, "/")
	fileName := tokens[len(tokens)-1]
	relPath := strings.Join(tokens[:len(tokens)-1], "/")
	absPath := filepath.Join(config.CachePath, relPath)
	absFile := filepath.Join(absPath, fileName)
	fileExt := strings.ToLower(filepath.Ext(absFile))

	// Fail if we dont have a hash
	if item.InstallerItemHash == "" {
		gorillalog.Warn("Installer hash missing for item:", item.DisplayName)
		return
	}

	// If the file exists, check the hash
	var verified bool
	if _, err := os.Stat(absFile); err == nil {
		verified = download.Verify(absFile, item.InstallerItemHash)
	}

	// If hash failed, download the installer
	if !verified {
		gorillalog.Info("Downloading", item.DisplayName)
		// Download the installer
		installerURL := config.Current.URL + item.InstallerItemLocation
		err := download.File(absPath, installerURL)
		if err != nil {
			gorillalog.Warn("Unable to retrieve package:", item.InstallerItemLocation, err)
			return
		}
		verified = download.Verify(absFile, item.InstallerItemHash)
	}

	// Return if hash verification fails
	if !verified {
		gorillalog.Warn("Hash mismatch:", item.DisplayName)
		return
	}

	// Define the command and arguments based on the installer type
	var installCmd string
	var installArgs []string

	if fileExt == ".nupkg" {
		gorillalog.Info("Installing nupkg:", fileName)
		installCmd = filepath.Join(os.Getenv("ProgramData"), "chocolatey/bin/choco.exe")
		installArgs = []string{"install", absFile, "-y", "-r"}

	} else if fileExt == ".msi" {
		gorillalog.Info("Installing MSI:", fileName)
		installCmd = filepath.Join(os.Getenv("WINDIR"), "system32/", "msiexec.exe")
		installArgs = []string{"/i", absFile, "/qn", "/norestart"}

	} else if fileExt == ".exe" {
		gorillalog.Info("Installing exe installer:", fileName)
		installCmd = absFile
		installArgs = item.InstallerItemArguments

	} else if fileExt == ".ps1" {
		gorillalog.Info("Installing Powershell script:", fileName)
		installCmd = filepath.Join(os.Getenv("WINDIR"), "system32/", "WindowsPowershell", "v1.0", "powershell.exe")
		installArgs = []string{"-NoProfile", "-NoLogo", "-NonInteractive", "-WindowStyle", "Normal", "-ExecutionPolicy", "Bypass", "-File", absFile}

	} else {
		gorillalog.Warn("Unable to install:", fileName)
		gorillalog.Warn("Installer type unsupported:", fileExt)
		return
	}

	// Add the item to UpdatedItems in GorillaReport
	report.UpdatedItems = append(report.UpdatedItems, item)

	// Run the command and arguments
	runCommand(installCmd, installArgs)

	return
}
