// Without a darwin specific build, tests will try to include Windows libraries and fail

// +build !windows

package status

import (
	"fmt"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
)

// CheckStatus determines the method for checking status
func CheckStatus(catalogItem catalog.Item, installType string) (install bool, checkErr error) {

	// Catch special names used in tests
	if catalogItem.DisplayName == "_gorilla_dev_action_noerror_" {
		gorillalog.Warn("Running Development Tests!")
		gorillalog.Warn(catalogItem.DisplayName)
		return true, nil
	} else if catalogItem.DisplayName == "_gorilla_dev_noaction_noerror_" {
		gorillalog.Warn("Running Development Tests!")
		gorillalog.Warn(catalogItem.DisplayName)
		return false, nil
	} else if catalogItem.DisplayName == "_gorilla_dev_action_error_" {
		gorillalog.Warn("Running Development Tests!")
		gorillalog.Warn(catalogItem.DisplayName)
		return true, fmt.Errorf("testing %v", catalogItem.DisplayName)
	} else if catalogItem.DisplayName == "_gorilla_dev_noaction_error_" {
		gorillalog.Warn("Running Development Tests!")
		gorillalog.Warn(catalogItem.DisplayName)
		return false, fmt.Errorf("testing %v", catalogItem.DisplayName)
	}

	fmt.Println(catalogItem.DisplayName)
	fmt.Println(installType)

	gorillalog.Error("Gorilla only works on Windows!")
	return false, nil
}
