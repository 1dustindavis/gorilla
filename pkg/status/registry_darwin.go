// Without a darwin specific build, go tools will try to include Windows libraries and fail

// +build !windows

package status

import "github.com/1dustindavis/gorilla/pkg/catalog"

func getUninstallKeys() map[string]Application {
	return nil
}

func checkRegistry(catalogItem catalog.Item, installType string) (actionNeeded bool, checkErr error) {
	return false, nil
}
