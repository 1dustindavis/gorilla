//go:build windows
// +build windows

package status

import (
	"fmt"

	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	"github.com/gonutz/w32"
)

//
//
// This is based on the StackOverflow answer here: https://stackoverflow.com/a/46222037
// Information on what strings can be passed to `VerQueryValueString` can
// be found here: https://docs.microsoft.com/en-us/windows/desktop/api/winver/nf-winver-verqueryvaluea#remarks
//
// This uses a bunch of C stuff I dont understand and seems kinda silly.
// Here is what I *think* I understand:
// - GetFileVersionInfoSize returns the size of the buffer needed to store file metadata
// - GetFileVersionInfo returns the actual metadata, but only the "fixed" portion is directly readable
// - VerQueryValueRoot gets info from he fixed part, which contains the "FileVersion"
// - VerQueryValueTranslations The other stuff is in a "non-fixed" part, and this function translates it ¯\_(ツ)_/¯
// - VerQueryValueString looks up data in the translated data
//
// Again, I dont really understand this, so maybe this is all wrong...
//
//

// GetFileMetadata gets Windows metadata from the provided path
// Returns a `WindowsMetadata` struct as defined in `status.go`
func GetFileMetadata(path string) WindowsMetadata {

	// finalMetadata is a struct for us to store our return values in
	var finalMetadata WindowsMetadata

	// bufferSize is the size of data needed to store the metadata
	// zero means there is no metadata
	bufferSize := w32.GetFileVersionInfoSize(path)
	if bufferSize <= 0 {
		gorillalog.Info("No metadata found:", path)
		return finalMetadata
	}

	// rawMetadata will store the returned metadata
	rawMetadata := make([]byte, bufferSize)
	ok := w32.GetFileVersionInfo(path, rawMetadata)
	if !ok {
		gorillalog.Warn("Unable to get metadata:", path)
		return finalMetadata
	}

	// fixed contains the "fixed" portion at the root of our raw metadata
	fixed, ok := w32.VerQueryValueRoot(rawMetadata)
	if !ok {
		gorillalog.Warn("Unable to get file version:", path)
		return finalMetadata
	}

	// rawVersion is the binary format of the file version
	rawVersion := fixed.FileVersion()

	// Convert the individual version segments to integers
	finalMetadata.versionMajor = int(rawVersion & 0xFFFF000000000000 >> 48)
	finalMetadata.versionMinor = int(rawVersion & 0x0000FFFF00000000 >> 32)
	finalMetadata.versionPatch = int(rawVersion & 0x00000000FFFF0000 >> 16)
	finalMetadata.versionBuild = int(rawVersion & 0x000000000000FFFF >> 0)

	// Combine everything into a pretty string
	finalMetadata.versionString = fmt.Sprintf(
		"%d.%d.%d.%d",
		finalMetadata.versionMajor,
		finalMetadata.versionMinor,
		finalMetadata.versionPatch,
		finalMetadata.versionBuild,
	)

	// translation is the non-fixed part after translation/processing(?)
	translation, ok := w32.VerQueryValueTranslations(rawMetadata)
	if !ok {
		gorillalog.Warn("Unable to get 'translate' metadata:", path)
		return finalMetadata
	}
	if len(translation) == 0 {
		gorillalog.Warn("Unable to get additional metadata:", path)
		return finalMetadata
	}
	translatedData := translation[0]

	// productName is the value of "ProductName" in the metadata
	finalMetadata.productName, ok = w32.VerQueryValueString(rawMetadata, translatedData, "ProductName")
	if !ok {
		gorillalog.Info("Unable to get product name from metadata:", path)
	}

	return finalMetadata

}
