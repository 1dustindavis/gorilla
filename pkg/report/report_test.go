package report

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

var (
	expectedItems = make(map[string]interface{})
)

// TestStart validates that a properly formated `Items` object
// is created with the expected starting data
func TestStart(t *testing.T) {

	// Set our expectations
	fakeTime = time.Now().UTC()
	expectedTime := fakeTime.Format("2006-01-02 15:04:05 -0700")

	expectedUser, userErr := user.Current()
	if userErr != nil {
		fmt.Println("Unable to determine expected user", userErr)
	}

	expectedHostname, hostErr := os.Hostname()
	if hostErr != nil {
		fmt.Println("Unable to determine current time", hostErr)
	}

	// Put our expectations in a map for comparison
	expectedItems["StartTime"] = fmt.Sprint(expectedTime)

	expectedItems["CurrentUser"] = fmt.Sprint(expectedUser.Username)

	expectedItems["HostName"] = fmt.Sprint(expectedHostname)

	// Run the `Start` function
	Start()

	// Compare the actual struct of items to what we expected
	mapsMatch := reflect.DeepEqual(expectedItems, Items)

	if !mapsMatch {
		t.Errorf("\n\nExpected:\n\n%#v\n\nReceived:\n\n %#v", expectedItems, Items)
	}
}

// TestEnd validates that a properly formated `Items` object
// is updated with the correct items and
func TestEnd(t *testing.T) {

	// Set our expectations
	fakeTime = time.Now().UTC()
	expectedTime := fakeTime.Format("2006-01-02 15:04:05 -0700")
	expectedInstalls := []interface{}{"test Installs 1", "test Installs 2", "test Installs 3", "test Installs 4"}
	expectedUninstalls := []interface{}{"test Uninstalls 1", "test Uninstalls 2", "test Uninstalls 3", "test Uninstalls 4"}

	// Apend everything tp the correct lists
	InstalledItems = append(InstalledItems, expectedInstalls...)
	UninstalledItems = append(UninstalledItems, expectedUninstalls...)

	// Update the existing map for comparison
	expectedItems["EndTime"] = fmt.Sprint(expectedTime)
	expectedItems["InstalledItems"] = InstalledItems
	expectedItems["UninstalledItems"] = UninstalledItems

	// Run the `End` function
	End(t.TempDir())

	// Compare the actual results
	mapsMatch := reflect.DeepEqual(expectedItems, Items)

	if !mapsMatch {
		t.Errorf("\n\nExpected:\n\n%#v\n\nReceived:\n\n %#v", expectedItems, Items)
	}
}

func TestEndWritesReportToConfiguredAppDataPath(t *testing.T) {
	programData := t.TempDir()
	t.Setenv("ProgramData", programData)
	defaultPath := filepath.Join(programData, "gorilla")
	if err := os.MkdirAll(defaultPath, 0755); err != nil {
		t.Fatal(err)
	}
	customPath := t.TempDir()
	defaultReport := filepath.Join(defaultPath, "GorillaReport.json")
	if err := os.WriteFile(defaultReport, []byte("other configuration"), 0644); err != nil {
		t.Fatal(err)
	}

	End(customPath)
	if _, err := os.Stat(filepath.Join(customPath, "GorillaReport.json")); err != nil {
		t.Fatalf("custom report was not written: %v", err)
	}
	content, err := os.ReadFile(defaultReport)
	if err != nil || string(content) != "other configuration" {
		t.Fatalf("report from another configuration was overwritten: %q, %v", content, err)
	}

	End("")
	content, err = os.ReadFile(defaultReport)
	if err != nil || string(content) == "other configuration" {
		t.Fatalf("empty app data path did not retain the default destination: %q, %v", content, err)
	}
}
