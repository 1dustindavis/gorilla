package status

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/download"
	"github.com/hashicorp/go-version"
	"golang.org/x/sys/windows/registry"
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

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func getUninstallKeys() map[string]Application {

	// Get the Uninstall key from HKLM
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\Uninstall`, registry.READ)
	if err != nil {
		log.Fatal("Unable to read registry key", err)
	}
	defer key.Close()

	// Get all the subkeys under Uninstall
	subKeys, err := key.ReadSubKeyNames(0)
	if err != nil {
		log.Fatal("Unable to read registry sub key:", err)
	}

	installedItems := make(map[string]Application)
	// Get the details of each subkey
	for _, item := range subKeys {
		var installedItem Application
		itemKeyName := `Software\Microsoft\Windows\CurrentVersion\Uninstall\` + item
		itemKey, err := registry.OpenKey(registry.LOCAL_MACHINE, itemKeyName, registry.READ)
		if err != nil {
			log.Fatal("Unable to read registry key", err)
		}
		defer key.Close()

		itemValues, err := itemKey.ReadValueNames(0)
		if stringInSlice("DisplayName", itemValues) && stringInSlice("DisplayVersion", itemValues) {
			installedItem.Key = itemKeyName
			installedItem.Name, _, err = itemKey.GetStringValue("DisplayName")
			if err != nil {
				log.Fatal("Unable to read DisplayName", err)
			}

			installedItem.Version, _, err = itemKey.GetStringValue("DisplayVersion")
			if err != nil {
				log.Fatal("Unable to read DisplayVersion", err)
			}

			installedItem.Uninstall, _, err = itemKey.GetStringValue("UninstallString")
			if err != nil {
				log.Fatal("Unable to read UninstallString", err)
			}
			installedItems[installedItem.Name] = installedItem
		}

	}
	return installedItems
}

func checkScript(catalogItem catalog.Item) (install bool, checkErr error) {

	// Write InstallCheckScript to disk as a Powershell file
	tmpScript := filepath.Join(config.CachePath, "tmpCheckScript.ps1")
	ioutil.WriteFile(tmpScript, []byte(catalogItem.InstallCheckScript), 0755)

	// Build the command to execute the script
	psCmd := filepath.Join(os.Getenv("WINDIR"), "system32/", "WindowsPowershell", "v1.0", "powershell.exe")
	psArgs := []string{"-NoProfile", "-NoLogo", "-NonInteractive", "-WindowStyle", "Normal", "-ExecutionPolicy", "Bypass", "-File", tmpScript}

	// Execute the script
	cmd := exec.Command(psCmd, psArgs...)
	stdOut, stdErr := cmd.CombinedOutput()

	// Delete the temporary sctip
	os.Remove(tmpScript)

	if config.Verbose {
		fmt.Println("stdout:")
		fmt.Println(stdOut)
		fmt.Println("stderr:")
		fmt.Println(stdErr)
	}

	// Dont't install if stderr is zero
	if stdErr == nil {
		install = false
	} else {
		install = true
	}

	return install, checkErr
}

func checkPath(catalogItem catalog.Item) (install bool, checkErr error) {
	path := filepath.Clean(catalogItem.InstallCheckPath)
	hash := catalogItem.InstallCheckPathHash
	if config.Verbose {
		fmt.Println(path)
	}

	// Confirm that path exists
	// Install if we get an error
	if _, checkErr := os.Stat(path); checkErr != nil {
		install = true
	} else {
		install = false
	}

	// If a hash is configured, verify it matches
	if !install && hash != "" {
		install = download.Verify(path, hash)
	}

	return install, checkErr
}

// checkRegistry iterates through the local registry and compiles all installed software
func checkRegistry(catalogItem catalog.Item, installType string) (install bool, checkErr error) {
	// Iterate through the reg keys to compare with the catalog
	catalogVersion, err := version.NewVersion(catalogItem.Version)
	if err != nil {
		fmt.Println("Unable to parse new version: ", catalogItem.DisplayName, err)
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
				fmt.Println("Unable to parse current version", err)
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
		install = false
	} else if installType == "uninstall" && installed {
		install = false
	} else if installed && versionMatch {
		install = false
	} else {
		install = true
	}

	return install, checkErr

}

// CheckStatus determines the method for checking status
func CheckStatus(catalogItem catalog.Item, installType string) (install bool, checkErr error) {

	if catalogItem.InstallCheckScript != "" {
		fmt.Printf("Checking status of %s via Script...\n", catalogItem.DisplayName)
		return checkScript(catalogItem)

	} else if catalogItem.InstallCheckPath != "" {
		fmt.Printf("Checking status of %s via Path...\n", catalogItem.DisplayName)
		return checkPath(catalogItem)

	}

	// If needed, populate applications status from the registry
	if len(RegistryItems) == 0 {
		RegistryItems = getUninstallKeys()
	}

	fmt.Printf("Checking status of %s via Registry...\n", catalogItem.DisplayName)
	return checkRegistry(catalogItem, installType)
}
