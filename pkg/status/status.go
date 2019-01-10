package status

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/download"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
)

// Application Contiains attributes for an installed application
type Application struct {
	Key       string
	Location  string
	Name      string
	Source    string
	Uninstall string
	Version   string
}

var (
	// RegistryItems contains the status of all of the applications in the registry
	RegistryItems map[string]Application
)

func checkScript(catalogItem catalog.Item) (actionNeeded bool, checkErr error) {

	// Write InstallCheckScript to disk as a Powershell file
	tmpScript := filepath.Join(config.CachePath, "tmpCheckScript.ps1")
	ioutil.WriteFile(tmpScript, []byte(catalogItem.InstallCheckScript), 0755)

	// Build the command to execute the script
	psCmd := filepath.Join(os.Getenv("WINDIR"), "system32/", "WindowsPowershell", "v1.0", "powershell.exe")
	psArgs := []string{"-NoProfile", "-NoLogo", "-NonInteractive", "-WindowStyle", "Normal", "-ExecutionPolicy", "Bypass", "-File", tmpScript}

	// Execute the script
	cmd := exec.Command(psCmd, psArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	cmdSuccess := cmd.ProcessState.Success()
	outStr, errStr := string(stdout.Bytes()), string(stderr.Bytes())

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
	path := filepath.Clean(catalogItem.InstallCheckPath)
	hash := catalogItem.InstallCheckPathHash
	gorillalog.Debug("Check Path", path)

	// Confirm that path exists
	// Install if we get an error
	if _, checkErr := os.Stat(path); checkErr != nil {
		actionNeeded = true
	} else {
		actionNeeded = false
	}

	// If a hash is configured, verify it matches
	if !actionNeeded && hash != "" {
		actionNeeded = download.Verify(path, hash)
	}

	return actionNeeded, checkErr
}

// CheckStatus determines the method for checking status
func CheckStatus(catalogItem catalog.Item, installType string) (actionNeeded bool, checkErr error) {

	if catalogItem.InstallCheckScript != "" {
		gorillalog.Info("Checking status via Script:", catalogItem.DisplayName)
		return checkScript(catalogItem)

	} else if catalogItem.InstallCheckPath != "" {
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
