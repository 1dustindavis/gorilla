package report

import (
	"fmt"
	"os"
	"os/user"
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

	// Put our expectations in a map for comparisions
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
	expectedUpdates := []interface{}{"test Updates 1", "test Updates 2", "test Updates 3", "test Updates 4"}

	// Apend everything tp the correct lists
	InstalledItems = append(InstalledItems, expectedInstalls...)
	UninstalledItems = append(UninstalledItems, expectedUninstalls...)
	UpdatedItems = append(UpdatedItems, expectedUpdates...)

	// Update the existing map for comparison
	expectedItems["EndTime"] = fmt.Sprint(expectedTime)
	expectedItems["InstalledItems"] = InstalledItems
	expectedItems["UninstalledItems"] = UninstalledItems
	expectedItems["UpdatedItems"] = UpdatedItems

	// Run the `End` function
	End()

	// Compare the actual results
	mapsMatch := reflect.DeepEqual(expectedItems, Items)

	if !mapsMatch {
		t.Errorf("\n\nExpected:\n\n%#v\n\nReceived:\n\n %#v", expectedItems, Items)
	}
}
