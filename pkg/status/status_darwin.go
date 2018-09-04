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

	fmt.Println(catalogItem.DisplayName)
	fmt.Println(installType)

	gorillalog.Error("Gorilla only works on Windows!")
	return false, nil
}
