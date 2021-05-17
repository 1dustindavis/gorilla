// Without a darwin specific build, go tools will try to include Windows libraries and fail

// +build !windows

package status

import (
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
)

// GetFileMetadata is just a placeholder on darwin
func GetFileMetadata(path string) WindowsMetadata {
	// Log a warning since we are not running on windows
	gorillalog.Warn("GetFileMetadata only supported on Windows:", path)

	// Set a fake `productName` and `versionString`
	var fakeMetadata WindowsMetadata
	fakeMetadata.productName = "Gorilla Test"
	fakeMetadata.companyName = "Gorilla Inc"
	fakeMetadata.versionString = "3.2.0.1"
	fakeMetadata.versionMajor = 3
	fakeMetadata.versionMinor = 2
	fakeMetadata.versionPatch = 0
	fakeMetadata.versionBuild = 1

	// Return the struct that holds our message
	return fakeMetadata
}
