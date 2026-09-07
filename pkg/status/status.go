package status

import (
	"bytes"
	"fmt"
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
	actionNeeded, _, _, checkErr = checkRegistryEvidence(catalogItem, installType)
	return
}

func checkRegistryEvidence(catalogItem catalog.Item, installType string) (actionNeeded bool, installed bool, installedVersion string, checkErr error) {
	// Iterate through the reg keys to compare with the catalog
	checkReg := catalogItem.Check.Registry
	catalogVersion, err := version.NewVersion(checkReg.Version)
	if err != nil {
		gorillalog.Warn("Unable to parse new version: ", checkReg.Version, err)
		return true, false, "", err
	}

	gorillalog.Debug("Check registry version:", checkReg.Version)
	// If needed, populate applications status from the registry
	if len(RegistryItems) == 0 {
		RegistryItems, checkErr = getUninstallKeys()
	}

	var versionMatch bool
	for _, regItem := range RegistryItems {
		// Check if the catalog name is in the registry
		if strings.Contains(regItem.Name, checkReg.Name) {
			installed = true
			installedVersion = regItem.Version
			gorillalog.Debug("Current installed version:", regItem.Version)

			// Check if the catalog version matches the registry
			currentVersion, err := version.NewVersion(regItem.Version)
			if err != nil {
				gorillalog.Warn("Unable to parse current version", err)
				return true, true, installedVersion, err
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

	return actionNeeded, installed, installedVersion, checkErr
}

func checkScript(catalogItem catalog.Item, cachePath string, installType string) (actionNeeded bool, checkErr error) {
	if err := os.MkdirAll(cachePath, 0755); err != nil {
		return false, err
	}

	// Write InstallCheckScript to disk as a Powershell file
	tmpScript := filepath.Join(cachePath, "tmpCheckScript.ps1")
	if err := os.WriteFile(tmpScript, []byte(catalogItem.Check.Script), 0755); err != nil {
		return false, err
	}

	// Build the command to execute the script
	psCmd := filepath.Join(os.Getenv("WINDIR"), "system32/", "WindowsPowershell", "v1.0", "powershell.exe")
	psArgs := []string{"-NoProfile", "-NoLogo", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", tmpScript}

	// Execute the script
	cmd := execCommand(psCmd, psArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	cmdSuccess := cmd.ProcessState != nil && cmd.ProcessState.Success()
	outStr, errStr := stdout.String(), stderr.String()

	// Delete the temporary script
	if err := os.Remove(tmpScript); err != nil && !os.IsNotExist(err) {
		gorillalog.Warn("Unable to remove temporary check script:", tmpScript, err)
	}

	// Log results
	gorillalog.Debug("Command Error:", err)
	gorillalog.Debug("stdout:", outStr)
	gorillalog.Debug("stderr:", errStr)

	if cmd.ProcessState == nil {
		return false, err
	}

	actionNeeded = false
	// Application not installed if exit 0
	if installType == "uninstall" {
		actionNeeded = !cmdSuccess
	} else if installType == "install" || installType == "update" {
		actionNeeded = cmdSuccess
	}

	return actionNeeded, checkErr
}

func checkPath(catalogItem catalog.Item, installType string) (actionNeeded bool, checkErr error) {
	actionNeeded, _, _, _, checkErr = checkPathEvidence(catalogItem, installType)
	return
}

func checkPathEvidence(catalogItem catalog.Item, installType string) (actionNeeded bool, state ObservedState, installedVersion, detail string, checkErr error) {
	checks := catalogItem.Check.File
	if len(checks) == 0 {
		return false, Unknown, "", "no_check", nil
	}
	present, missing := 0, 0
	versionConflict, versionOutdated := false, false
	actionDecided := false
	for _, checkFile := range checks {
		path := filepath.Clean(checkFile.Path)
		gorillalog.Debug("Check file path:", path)
		_, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				missing++
				if !actionDecided {
					if installType == "install" {
						actionNeeded = true
					}
					actionDecided = true
				}
				continue
			}
			return actionNeeded, DetectionFailed, "", "check_failed", err
		}
		present++
		if !actionDecided && installType == "uninstall" {
			actionNeeded = true
		}
		if checkFile.Hash != "" && !download.Verify(path, checkFile.Hash) {
			if !actionDecided {
				actionNeeded, actionDecided = true, true
			}
			detail = "non_version_requirement_unsatisfied"
		}
		if checkFile.Version == "" {
			continue
		}
		found := GetFileMetadata(path).versionString
		if found == "" {
			actionDecided = true
			if detail == "" {
				detail = "installed_version_unavailable"
			}
			continue
		}
		if installedVersion != "" && installedVersion != found {
			versionConflict = true
		}
		installedVersion = found
		have, err := version.NewVersion(found)
		if err != nil {
			actionNeeded = true
			return true, DetectionFailed, "", "check_failed", err
		}
		want, err := version.NewVersion(checkFile.Version)
		if err != nil {
			actionNeeded = true
			return true, DetectionFailed, "", "check_failed", err
		}
		if have.LessThan(want) {
			if !actionDecided {
				actionNeeded = true
			}
			actionDecided, versionOutdated = true, true
		}
	}
	if present == 0 && missing == len(checks) {
		if installType != "install" {
			actionNeeded = false
		}
		return actionNeeded, Absent, "", "", nil
	}
	if present != len(checks) {
		return actionNeeded, Unknown, "", "partial_file_evidence", nil
	}
	if versionConflict {
		installedVersion = ""
	}
	if versionOutdated {
		return actionNeeded, UpdateAvailable, installedVersion, detail, nil
	}
	return actionNeeded, Installed, installedVersion, detail, nil
}

// checkAppx checks whether an AppX/MSIX package is provisioned and at the required version.
// It calls `Get-AppxProvisionedPackage -Online` via PowerShell and parses the Version field
// from the output to compare against the catalog version.
func checkAppx(catalogItem catalog.Item, installType string) (actionNeeded bool, checkErr error) {
	actionNeeded, _, checkErr = checkAppxEvidence(catalogItem, installType)
	return
}

func checkAppxEvidence(catalogItem catalog.Item, installType string) (actionNeeded bool, installedVersionStr string, checkErr error) {
	checkAppxItem := catalogItem.Check.Appx
	psCmd := filepath.Join(os.Getenv("WINDIR"), "system32/", "WindowsPowershell", "v1.0", "powershell.exe")
	psArgs := []string{
		"-NoProfile", "-NoLogo", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-Command",
		fmt.Sprintf(
			"$p = Get-AppxProvisionedPackage -Online | Where-Object { $_.DisplayName -eq '%s' }; if ($p) { $p.Version } else { '' }",
			checkAppxItem.Name,
		),
	}

	cmd := execCommand(psCmd, psArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		gorillalog.Warn("checkAppx command error:", err)
		return installType == "install", "", err
	}

	installedVersionStr = strings.TrimSpace(stdout.String())
	gorillalog.Debug("AppX installed version:", installedVersionStr)
	gorillalog.Debug("AppX stderr:", stderr.String())

	installed := installedVersionStr != ""

	var versionMatch bool
	if installed && checkAppxItem.Version != "" {
		catalogVersion, err := version.NewVersion(checkAppxItem.Version)
		if err != nil {
			gorillalog.Warn("Unable to parse catalog appx version:", checkAppxItem.Version, err)
			return true, installedVersionStr, err
		}
		currentVersion, err := version.NewVersion(installedVersionStr)
		if err != nil {
			gorillalog.Warn("Unable to parse installed appx version:", installedVersionStr, err)
			return true, installedVersionStr, err
		}
		versionMatch = !currentVersion.LessThan(catalogVersion)
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

	return actionNeeded, installedVersionStr, checkErr
}

// CheckStatus determines the method for checking status
func CheckStatus(catalogItem catalog.Item, installType, cachePath string) (actionNeeded bool, checkErr error) {
	result, err := observe(catalogItem, installType, cachePath, false)
	return result.ActionNeeded, err
}

// Observe runs the same selected check used by CheckStatus and also returns the
// evidence needed by service consumers. Selection precedence is intentionally
// identical to the historical CheckStatus implementation.
func Observe(catalogItem catalog.Item, installType, cachePath string) (Observation, error) {
	return observe(catalogItem, installType, cachePath, true)
}

// conservativeRegistry affects only the evidence returned for ambiguous
// registry substring matches. CheckStatus keeps its historical first-match
// action decision; the richer service observation must not expose that
// nondeterministic match as authoritative presence or version evidence.
func observe(catalogItem catalog.Item, installType, cachePath string, conservativeRegistry bool) (Observation, error) {

	if catalogItem.Check.Script != "" {
		gorillalog.Info("Checking status via script:", catalogItem.DisplayName)
		actionNeeded, err := checkScript(catalogItem, cachePath, installType)
		state := Unknown
		detail := "script_requirement_satisfied"
		if actionNeeded {
			detail = "script_requirement_not_satisfied"
		}
		if err != nil {
			state, detail = DetectionFailed, "check_failed"
		}
		return observation(state, "", detail, actionNeeded), err

	} else if catalogItem.Check.File != nil {
		gorillalog.Info("Checking status via file:", catalogItem.DisplayName)
		actionNeeded, state, installedVersion, detail, err := checkPathEvidence(catalogItem, installType)
		return observation(state, installedVersion, detail, actionNeeded), err

	} else if catalogItem.Check.Registry.Version != "" {
		gorillalog.Info("Checking status via registry:", catalogItem.DisplayName)
		if conservativeRegistry {
			return observeRegistry(catalogItem, installType)
		}
		actionNeeded, installed, installedVersion, err := checkRegistryEvidence(catalogItem, installType)
		if err != nil {
			return observation(DetectionFailed, "", "check_failed", actionNeeded), err
		}
		if installed {
			state := Installed
			if actionNeeded && installType != "uninstall" {
				state = UpdateAvailable
			}
			return observation(state, installedVersion, "", actionNeeded), nil
		}
		return observation(Absent, "", "", actionNeeded), nil

	} else if catalogItem.Check.Appx.Name != "" {
		gorillalog.Info("Checking status via appx:", catalogItem.DisplayName)
		actionNeeded, installedVersion, err := checkAppxEvidence(catalogItem, installType)
		if err != nil {
			return observation(DetectionFailed, "", "check_failed", actionNeeded), err
		}
		if installedVersion == "" {
			return observation(Absent, "", "", actionNeeded), nil
		}
		state := Installed
		if actionNeeded && installType != "uninstall" && catalogItem.Check.Appx.Version != "" {
			state = UpdateAvailable
		}
		return observation(state, installedVersion, "", actionNeeded), nil
	}

	gorillalog.Warn("Not enough data to check the current status:", catalogItem.DisplayName)
	return observation(Unknown, "", "no_check", false), nil

}

func observeRegistry(catalogItem catalog.Item, installType string) (Observation, error) {
	checkReg := catalogItem.Check.Registry
	wantedVersion, err := version.NewVersion(checkReg.Version)
	if err != nil {
		return observation(DetectionFailed, "", "check_failed", true), err
	}
	if len(RegistryItems) == 0 {
		RegistryItems, err = getUninstallKeys()
		if err != nil {
			return observation(DetectionFailed, "", "check_failed", true), err
		}
	}

	matches := make([]RegistryApplication, 0, 1)
	for _, item := range RegistryItems {
		if strings.Contains(item.Name, checkReg.Name) {
			matches = append(matches, item)
		}
	}
	if len(matches) == 0 {
		actionNeeded := installType == "install"
		return observation(Absent, "", "", actionNeeded), nil
	}
	if len(matches) > 1 {
		return observation(Unknown, "", "ambiguous_registry_match", false), nil
	}

	installedVersion := matches[0].Version
	currentVersion, err := version.NewVersion(installedVersion)
	if err != nil {
		return observation(DetectionFailed, "", "check_failed", true), err
	}
	versionSatisfied := !currentVersion.LessThan(wantedVersion)
	actionNeeded := !versionSatisfied
	if installType == "uninstall" {
		actionNeeded = true
	}
	state := Installed
	if !versionSatisfied && installType != "uninstall" {
		state = UpdateAvailable
	}
	return observation(state, installedVersion, "", actionNeeded), nil
}
