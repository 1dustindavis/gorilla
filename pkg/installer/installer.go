package installer

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/download"
	"github.com/1dustindavis/gorilla/pkg/status"
)

func alreadyInstalled(catalogItem catalog.Item) bool {
	installed, versionMatch, err := status.CheckRegistry(catalogItem)
	if err != nil {
		fmt.Println("Unable to check status of item:", catalogItem.DisplayName)
		return false
	}
	if installed && versionMatch {
		fmt.Println(catalogItem.DisplayName, "already installed.")
		return true
	}
	return false
}

// Install runs the installer
func Install(item catalog.Item, cachePath string, verbose bool, repoURL string) {

	// Check if the item is currently installed and up-to-date
	if alreadyInstalled(item) {
		return
	}

	// Get all the path strings we will need
	tokens := strings.Split(item.InstallerItemLocation, "/")
	fileName := tokens[len(tokens)-1]
	relPath := strings.Join(tokens[:len(tokens)-1], "/")
	absPath := filepath.Join(cachePath, relPath)
	absFile := filepath.Join(absPath, fileName)
	fileExt := strings.ToLower(filepath.Ext(absFile))

	// If the file exists, check the hash
	var verified bool
	if _, err := os.Stat(absFile); err == nil {
		verified = download.Verify(absFile, item.InstallerItemHash)
	}

	// If hash failed, download the installer
	if !verified {
		fmt.Printf("Downloading %s...\n", fileName)
		// Download the installer
		installerURL := repoURL + item.InstallerItemLocation
		err := download.File(absPath, installerURL)
		if err != nil {
			fmt.Println("Unable to retrieve package:", item.InstallerItemLocation, err)
			os.Exit(1)
		}
		verified = download.Verify(absFile, item.InstallerItemHash)
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
		installArgs = []string{"install", absFile, "-y", "-r"}

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
	if verbose {
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
