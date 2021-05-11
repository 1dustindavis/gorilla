package status

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/download"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	version "github.com/hashicorp/go-version"
)

// RegistryApplication contains attributes for an installed application
type RegistryApplication struct {
	Key       string
	Location  string
	Name      string
	Source    string
	Uninstall string
	Version   string
}

// WindowsMetadata contains extended metadata retrieved in the `properties.go`
type WindowsMetadata struct {
	productName   string
	companyName   string
	versionString string
	versionMajor  int
	versionMinor  int
	versionPatch  int
	versionBuild  int
}

var (
	// RegistryItems contains the status of all of the applications in the registry
	RegistryItems map[string]RegistryApplication

	// Abstracted functions so we can override these in unit tests
	execCommand = exec.Command
)

// checkRegistry iterates through the local registry and compiles all installed software
func checkRegistry(catalogItem catalog.Item, installType string) (actionNeeded bool, checkErr error) {
	// Iterate through the reg keys to compare with the catalog
	checkReg := catalogItem.Check.Registry
	catalogVersion, err := version.NewVersion(checkReg.Version)
	if err != nil {
		gorillalog.Warn("Unable to parse new version: ", checkReg.Version, err)
	}

	gorillalog.Debug("Check registry version:", checkReg.Version)
	// If needed, populate applications status from the registry
	if len(RegistryItems) == 0 {
		RegistryItems, checkErr = getUninstallKeys()
	}

	var installed bool
	var versionMatch bool
	for _, regItem := range RegistryItems {
		// Check if the catalog name is in the registry
		if strings.Contains(regItem.Name, checkReg.Name) {
			installed = true
			gorillalog.Debug("Current installed version:", regItem.Version)

			// Check if the catalog version matches the registry
			currentVersion, err := version.NewVersion(regItem.Version)
			if err != nil {
				gorillalog.Warn("Unable to parse current version", err)
			}
			outdated := currentVersion.LessThan(catalogVersion)
			if !outdated {
				versionMatch = true
			}
			break
		}

	}

	if installType == "update" && !installed {
		actionNeeded = false
	} else if installType == "uninstall" {
		actionNeeded = installed
	} else if installed && versionMatch {
		actionNeeded = false
	} else {
		actionNeeded = true
	}

	return actionNeeded, checkErr
}

func checkScript(catalogItem catalog.Item, cachePath string, installType string) (actionNeeded bool, checkErr error) {

	// Write InstallCheckScript to disk as a Powershell file
	tmpScript := filepath.Join(cachePath, "tmpCheckScript.ps1")
	ioutil.WriteFile(tmpScript, []byte(catalogItem.Check.Script), 0755)

	// Build the command to execute the script
	psCmd := filepath.Join(os.Getenv("WINDIR"), "system32/", "WindowsPowershell", "v1.0", "powershell.exe")
	psArgs := []string{"-NoProfile", "-NoLogo", "-NonInteractive", "-WindowStyle", "Normal", "-ExecutionPolicy", "Bypass", "-File", tmpScript}

	// Execute the script
	cmd := execCommand(psCmd, psArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	cmdSuccess := cmd.ProcessState.Success()
	outStr, errStr := stdout.String(), stderr.String()

	// Delete the temporary script
	os.Remove(tmpScript)

	// Log results
	gorillalog.Debug("Command Error:", err)
	gorillalog.Debug("stdout:", outStr)
	gorillalog.Debug("stderr:", errStr)

	actionNeeded = false
	// Application not installed if exit 0
	if installType == "uninstall" {
		actionNeeded = !cmdSuccess
	} else if installType == "install" {
		actionNeeded = cmdSuccess
	}

	return actionNeeded, checkErr
}

func checkPath(catalogItem catalog.Item, installType string) (actionNeeded bool, checkErr error) {
	var actionStore []bool

	// Iterate through all file provided paths
	for _, checkFile := range catalogItem.Check.File {
		path := filepath.Clean(checkFile.Path)
		gorillalog.Debug("Check file path", path)
		_, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {

				// when doing an install, and the file path does not exist
				// perform an install
				if installType == "install" {
					actionStore = append(actionStore, true)
					break
				}

				// When doing an update or uninstall, and the file path does
				// not exist, do nothing
				if installType == "update" || installType == "uninstall" {
					gorillalog.Debug(fmt.Sprintf("File path not found. Install type is %s, no action needed", installType))
					break
				}
			}
			gorillalog.Warn("Unable to check path:", path, err)
			break

		} else if err == nil {

			// When doing an uninstall, and the path exists
			// perform uninstall
			if installType == "uninstall" {
				actionStore = append(actionStore, true)
			}
		}

		// If a hash is not blank, verify it matches the file
		// if the hash does not match, we need to install
		if checkFile.Hash != "" {
			gorillalog.Debug("Check file hash:", checkFile.Hash)
			hashMatch := download.Verify(path, checkFile.Hash)
			if !hashMatch {
				actionStore = append(actionStore, true)
				break
			}
		}

		if checkFile.Version != "" {
			gorillalog.Debug("Check file version:", checkFile.Version)

			// Get the file metadata, and check that it has a value
			metadata := GetFileMetadata(path)
			if metadata.versionString == "" {
				break
			}
			gorillalog.Debug("Current installed version:", metadata.versionString)

			// Convert both strings to a `Version` object
			versionHave, err := version.NewVersion(metadata.versionString)
			if err != nil {
				gorillalog.Warn("Unable to compare version:", metadata.versionString)
				actionStore = append(actionStore, true)
				break
			}
			versionWant, err := version.NewVersion(checkFile.Version)
			if err != nil {
				gorillalog.Warn("Unable to compare version:", checkFile.Version)
				actionStore = append(actionStore, true)
				break
			}

			// Compare the versions
			outdated := versionHave.LessThan(versionWant)
			if outdated {
				actionStore = append(actionStore, true)
				break
			}
		}
	}

	for _, item := range actionStore {
		if item == true {
			actionNeeded = true
			return
		}
	}
	actionNeeded = false
	return actionNeeded, checkErr
}

// CheckStatus determines the method for checking status
func CheckStatus(catalogItem catalog.Item, installType, cachePath string) (actionNeeded bool, checkErr error) {

	if catalogItem.Check.Script != "" {
		gorillalog.Info("Checking status via script:", catalogItem.DisplayName)
		return checkScript(catalogItem, cachePath, installType)

	} else if catalogItem.Check.File != nil {
		gorillalog.Info("Checking status via file:", catalogItem.DisplayName)
		return checkPath(catalogItem, installType)

	} else if catalogItem.Check.Registry.Version != "" {
		gorillalog.Info("Checking status via registry:", catalogItem.DisplayName)
		return checkRegistry(catalogItem, installType)
	}

	gorillalog.Warn("Not enough data to check the current status:", catalogItem.DisplayName)
	return

}
