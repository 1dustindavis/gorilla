package status

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
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

// WindowsMetadata contains extended metadata retreived in the `properties.go`
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
	catalogVersion, err := version.NewVersion(catalogItem.Version)
	if err != nil {
		gorillalog.Warn("Unable to parse new version: ", catalogItem.DisplayName, err)
	}

	var installed bool
	var versionMatch bool
	for _, regItem := range RegistryItems {
		// Check if the catalog name is in the registry
		if strings.Contains(regItem.Name, catalogItem.DisplayName) {
			installed = true

			// Check if the catalog version matches the registry
			currentVersion, err := version.NewVersion(regItem.Version)
			if err != nil {
				gorillalog.Warn("Unable to parse current version", err)
			}
			if !currentVersion.LessThan(catalogVersion) {
				versionMatch = true
			}
			break
		}

	}

	// If we don't have version information, we can't compare
	if catalogItem.Version == "" {
		versionMatch = true
	}

	if installType == "update" && !installed {
		actionNeeded = false
	} else if installType == "uninstall" && installed {
		actionNeeded = true
	} else if installed && versionMatch {
		actionNeeded = false
	} else {
		actionNeeded = true
	}

	return actionNeeded, checkErr
}

func checkScript(catalogItem catalog.Item) (actionNeeded bool, checkErr error) {

	// Write InstallCheckScript to disk as a Powershell file
	tmpScript := filepath.Join(config.CachePath, "tmpCheckScript.ps1")
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

	// Install if exit 0
	actionNeeded = cmdSuccess

	return actionNeeded, checkErr
}

func checkPath(catalogItem catalog.Item) (actionNeeded bool, checkErr error) {
	path := filepath.Clean(catalogItem.Check.Path.Path)
	hash := catalogItem.Check.Path.Hash
	gorillalog.Debug("Check Path", path)

	// Just for testing!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
	gorillalog.Debug(GetFileMetadata(path))

	// Default to no action needed
	actionNeeded = false

	// Confirm that path exists
	// if we get an error, we need to install
	_, checkErr = os.Stat(path)
	if checkErr != nil {
		gorillalog.Debug("Path check failed for ", path)
		actionNeeded = true
		return
	}

	// If a hash is not blank, verify it matches the file
	// if the hash does not match, we need to install
	if hash != "" {
		gorillalog.Debug("Check Hash", hash)
		hashMatch := download.Verify(path, hash)
		if !hashMatch {
			actionNeeded = true
			return
		}
	}

	return actionNeeded, checkErr
}

// CheckStatus determines the method for checking status
func CheckStatus(catalogItem catalog.Item, installType string) (actionNeeded bool, checkErr error) {

	if catalogItem.Check.Script != "" {
		gorillalog.Info("Checking status via Script:", catalogItem.DisplayName)
		return checkScript(catalogItem)

	} else if catalogItem.Check.Path.Path != "" {
		gorillalog.Info("Checking status via Path:", catalogItem.DisplayName)
		return checkPath(catalogItem)
	}

	// If needed, populate applications status from the registry
	if len(RegistryItems) == 0 {
		RegistryItems = getUninstallKeys()
	}

	gorillalog.Info("Checking status via Registry:", catalogItem.DisplayName)
	return checkRegistry(catalogItem, installType)
}
