package status

import (
	"fmt"

	"github.com/1dustindavis/gorilla/pkg/gorillalog"
)

// GetFileMetadata is just a placeholder on darwin
func GetFileMetadata(path string) WindowsMetadata {
	// Create a message we can log and return
	msg := fmt.Sprintf("GetFileMetadata only supported on Windows: %v", path)

	// Log the message
	gorillalog.Warn(msg)

	// Put the message into a struct as the `productName`
	var fakeMetadata WindowsMetadata
	fakeMetadata.productName = msg

	// Return the struct that holds our message
	return fakeMetadata
}
