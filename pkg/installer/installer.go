package installer

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/download"
	"github.com/1dustindavis/gorilla/pkg/status"
)

// runCommand executes a command and it's argurments in the CMD enviroment
func runCommand(command string, arguments []string, verbose bool) {
	cmd := exec.Command(command, arguments...)
	cmdReader, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("command:", command, arguments)
		fmt.Fprintln(os.Stderr, "Error creating pipe to stdout", err)
		os.Exit(1)
	}

	scanner := bufio.NewScanner(cmdReader)
	if verbose {
		fmt.Println("command:", command, arguments)
		go func() {
			for scanner.Scan() {
				fmt.Printf("Command output | %s\n", scanner.Text())
			}
		}()
	}

	err = cmd.Start()
	if err != nil {
		fmt.Println("command:", command, arguments)
		fmt.Println(os.Stderr, "Error running command:", err)
		os.Exit(1)
	}

	err = cmd.Wait()
	if err != nil {
		fmt.Println("command:", command, arguments)
		fmt.Println(os.Stderr, "Command error:", err)
		os.Exit(1)
	}
	return
}

// Install runs the installer
func Install(item catalog.Item, cachePath string, verbose bool, repoURL string) {

	// Check the items current status
	install, err := status.CheckStatus(item, "install")
	if err != nil {
		fmt.Println("Unable to check status of ", item.DisplayName)
		return
	}

	if !install {
		return
	}

	// Get all the path strings we will need
	tokens := strings.Split(item.InstallerItemLocation, "/")
	fileName := tokens[len(tokens)-1]
	relPath := strings.Join(tokens[:len(tokens)-1], "/")
	absPath := filepath.Join(cachePath, relPath)
	absFile := filepath.Join(absPath, fileName)
	fileExt := strings.ToLower(filepath.Ext(absFile))

	// Fail if we dont have a hash
	if item.InstallerItemHash == "" {
		log.Fatalln("Installer hash missing for item:", item.DisplayName)
	}
	// If the file exists, check the hash
	var verified bool
	if _, err := os.Stat(absFile); err == nil {
		verified = download.Verify(absFile, item.InstallerItemHash)
	}

	// If hash failed, download the installer
	if !verified {
		fmt.Printf("Downloading %s...\n", item.DisplayName)
		// Download the installer
		installerURL := repoURL + item.InstallerItemLocation
		err := download.File(absPath, installerURL)
		if err != nil {
			log.Fatalln("Unable to retrieve package:", item.InstallerItemLocation, err)
		}
		verified = download.Verify(absFile, item.InstallerItemHash)
	}

	// Return if hash verification fails
	if !verified {
		log.Println("Hash mismatch:", item.DisplayName)
		return
	}

	// Define the command and arguments based on the installer type
	var installCmd string
	var installArgs []string

	if fileExt == ".nupkg" {
		fmt.Println("Installing choco:", fileName)
		installCmd = filepath.Join(os.Getenv("ProgramData"), "chocolatey/bin/choco.exe")
		installArgs = []string{"install", absFile, "-y", "-r"}

	} else if fileExt == ".msi" {
		fmt.Println("Installing MSI for", fileName)
		installCmd = filepath.Join(os.Getenv("WINDIR"), "system32/", "msiexec.exe")
		installArgs = []string{"/i", absFile, "/qn", "/norestart"}

	} else if fileExt == ".exe" {
		fmt.Println("EXE support not added yet:", fileName)
		return
	} else if fileExt == ".ps1" {
		fmt.Println("Installing via Powershell:", fileName)
		installCmd = filepath.Join(os.Getenv("WINDIR"), "system32/", "WindowsPowershell", "v1.0", "powershell.exe")
		installArgs = []string{"-NoProfile", "-NoLogo", "-NonInteractive", "-WindowStyle", "Normal", "-ExecutionPolicy", "Bypass", "-File", absFile}

	} else {
		fmt.Println("Unable to install", fileName)
		fmt.Println("Installer type unsupported:", fileExt)
		return
	}

	runCommand(installCmd, installArgs, verbose)

	return
}

// Uninstall runs the uninstaller
func Uninstall(item catalog.Item, cachePath string, verbose bool, repoURL string) {

	// Check the items current status
	install, err := status.CheckStatus(item, "uninstall")
	if err != nil {
		fmt.Println("Unable to check status of ", item.DisplayName)
		return
	}

	if install {
		return
	}

	// Get all the path strings we will need
	tokens := strings.Split(item.InstallerItemLocation, "/")
	fileName := tokens[len(tokens)-1]
	relPath := strings.Join(tokens[:len(tokens)-1], "/")
	absPath := filepath.Join(cachePath, relPath)
	absFile := filepath.Join(absPath, fileName)

	// Fail if we dont have a hash
	if item.InstallerItemHash == "" {
		log.Fatalln("Installer hash missing for item:", item.DisplayName)
	}

	// If the file exists, check the hash
	var verified bool
	if _, err := os.Stat(absFile); err == nil {
		verified = download.Verify(absFile, item.InstallerItemHash)
	}

	// If hash failed, download the installer
	if !verified {
		fmt.Printf("Downloading %s...\n", item.DisplayName)
		// Download the installer
		installerURL := repoURL + item.InstallerItemLocation
		err := download.File(absPath, installerURL)
		if err != nil {
			log.Fatalln("Unable to retrieve package:", item.InstallerItemLocation, err)
		}
		verified = download.Verify(absFile, item.InstallerItemHash)
	}

	// Return if hash verification fails
	if !verified {
		log.Println("Hash mismatch:", item.DisplayName)
		return
	}

	// Define the command and arguments based on the installer type
	var uninstallCmd string
	var uninstallArgs []string

	if item.UninstallMethod == "choco" {
		fmt.Println("Uninstalling choco:", item.DisplayName)
		uninstallCmd = filepath.Join(os.Getenv("ProgramData"), "chocolatey/bin/choco.exe")
		uninstallArgs = []string{"uninstall", absFile, "-y", "-r"}

	} else if item.UninstallMethod == "msi" {
		fmt.Println("unnstalling MSI for", item.DisplayName)
		uninstallCmd = filepath.Join(os.Getenv("WINDIR"), "system32/", "msiexec.exe")
		uninstallArgs = []string{"/x", absFile, "/qn", "/norestart"}
	} else {
		fmt.Println("Unable to uninstall", item.DisplayName)
		fmt.Println("Installer type unsupported:", item.UninstallMethod)
		return
	}

	runCommand(uninstallCmd, uninstallArgs, verbose)

	return
}

// Update runs the installer if the item is already installed, but not up-to-date
func Update(item catalog.Item, cachePath string, verbose bool, repoURL string) {

	// Check the items current status
	install, err := status.CheckStatus(item, "update")
	if err != nil {
		fmt.Println("Unable to check status of ", item.DisplayName)
		return
	}

	if !install {
		return
	}

	// Get all the path strings we will need
	tokens := strings.Split(item.InstallerItemLocation, "/")
	fileName := tokens[len(tokens)-1]
	relPath := strings.Join(tokens[:len(tokens)-1], "/")
	absPath := filepath.Join(cachePath, relPath)
	absFile := filepath.Join(absPath, fileName)
	fileExt := strings.ToLower(filepath.Ext(absFile))

	// Fail if we dont have a hash
	if item.InstallerItemHash == "" {
		log.Fatalln("Installer hash missing for item:", item.DisplayName)
	}

	// If the file exists, check the hash
	var verified bool
	if _, err := os.Stat(absFile); err == nil {
		verified = download.Verify(absFile, item.InstallerItemHash)
	}

	// If hash failed, download the installer
	if !verified {
		fmt.Printf("Downloading %s...\n", item.DisplayName)
		// Download the installer
		installerURL := repoURL + item.InstallerItemLocation
		err := download.File(absPath, installerURL)
		if err != nil {
			log.Fatalln("Unable to retrieve package:", item.InstallerItemLocation, err)
		}
		verified = download.Verify(absFile, item.InstallerItemHash)
	}

	// Return if hash verification fails
	if !verified {
		log.Println("Hash mismatch:", item.DisplayName)
		return
	}

	// Define the command and arguments based on the installer type
	var installCmd string
	var installArgs []string

	if fileExt == ".nupkg" {
		fmt.Println("Installing choco:", fileName)
		installCmd = filepath.Join(os.Getenv("ProgramData"), "chocolatey/bin/choco.exe")
		installArgs = []string{"install", absFile, "-y", "-r"}

	} else if fileExt == ".msi" {
		fmt.Println("Installing MSI for", fileName)
		installCmd = filepath.Join(os.Getenv("WINDIR"), "system32/", "msiexec.exe")
		installArgs = []string{"/i", absFile, "/qn", "/norestart"}

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

	runCommand(installCmd, installArgs, verbose)

	return
}
