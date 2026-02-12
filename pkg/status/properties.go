//go:build windows
// +build windows

package status

import (
	"fmt"
	"unsafe"

	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	"golang.org/x/sys/windows"
)

var (
	versionDLL                 = windows.NewLazySystemDLL("version.dll")
	procGetFileVersionInfoSize = versionDLL.NewProc("GetFileVersionInfoSizeW")
	procGetFileVersionInfo     = versionDLL.NewProc("GetFileVersionInfoW")
	procVerQueryValue          = versionDLL.NewProc("VerQueryValueW")
)

type vsFixedFileInfo struct {
	Signature        uint32
	StrucVersion     uint32
	FileVersionMS    uint32
	FileVersionLS    uint32
	ProductVersionMS uint32
	ProductVersionLS uint32
	FileFlagsMask    uint32
	FileFlags        uint32
	FileOS           uint32
	FileType         uint32
	FileSubtype      uint32
	FileDateMS       uint32
	FileDateLS       uint32
}

type langAndCodePage struct {
	Lang     uint16
	CodePage uint16
}

func getFileVersionInfoSize(path string) uint32 {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0
	}
	size, _, _ := procGetFileVersionInfoSize.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
	)
	return uint32(size)
}

func getFileVersionInfo(path string, rawMetadata []byte) bool {
	if len(rawMetadata) == 0 {
		return false
	}

	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}

	ret, _, _ := procGetFileVersionInfo.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		uintptr(uint32(len(rawMetadata))),
		uintptr(unsafe.Pointer(&rawMetadata[0])),
	)
	return ret != 0
}

func verQueryValue(rawMetadata []byte, subBlock string) (uintptr, uint32, bool) {
	if len(rawMetadata) == 0 {
		return 0, 0, false
	}

	subBlockPtr, err := windows.UTF16PtrFromString(subBlock)
	if err != nil {
		return 0, 0, false
	}

	var valuePtr uintptr
	var valueLen uint32
	ret, _, _ := procVerQueryValue.Call(
		uintptr(unsafe.Pointer(&rawMetadata[0])),
		uintptr(unsafe.Pointer(subBlockPtr)),
		uintptr(unsafe.Pointer(&valuePtr)),
		uintptr(unsafe.Pointer(&valueLen)),
	)
	if ret == 0 || valuePtr == 0 {
		return 0, 0, false
	}

	return valuePtr, valueLen, true
}

// GetFileMetadata gets Windows metadata from the provided path
// Returns a `WindowsMetadata` struct as defined in `status.go`
func GetFileMetadata(path string) WindowsMetadata {
	var finalMetadata WindowsMetadata

	bufferSize := getFileVersionInfoSize(path)
	if bufferSize <= 0 {
		gorillalog.Info("No metadata found:", path)
		return finalMetadata
	}

	rawMetadata := make([]byte, bufferSize)
	if !getFileVersionInfo(path, rawMetadata) {
		gorillalog.Warn("Unable to get metadata:", path)
		return finalMetadata
	}

	valuePtr, _, ok := verQueryValue(rawMetadata, "\\")
	if !ok {
		gorillalog.Warn("Unable to get file version:", path)
		return finalMetadata
	}
	fixed := (*vsFixedFileInfo)(unsafe.Pointer(valuePtr))
	if fixed.Signature != 0xFEEF04BD {
		gorillalog.Warn("Invalid fixed metadata signature:", path)
		return finalMetadata
	}

	finalMetadata.versionMajor = int(fixed.FileVersionMS >> 16)
	finalMetadata.versionMinor = int(fixed.FileVersionMS & 0xFFFF)
	finalMetadata.versionPatch = int(fixed.FileVersionLS >> 16)
	finalMetadata.versionBuild = int(fixed.FileVersionLS & 0xFFFF)
	finalMetadata.versionString = fmt.Sprintf(
		"%d.%d.%d.%d",
		finalMetadata.versionMajor,
		finalMetadata.versionMinor,
		finalMetadata.versionPatch,
		finalMetadata.versionBuild,
	)

	valuePtr, valueLen, ok := verQueryValue(rawMetadata, "\\VarFileInfo\\Translation")
	if !ok {
		gorillalog.Warn("Unable to get 'translate' metadata:", path)
		return finalMetadata
	}
	translationSize := int(unsafe.Sizeof(langAndCodePage{}))
	if valueLen < uint32(translationSize) {
		gorillalog.Warn("Unable to get additional metadata:", path)
		return finalMetadata
	}
	translationCount := int(valueLen) / translationSize
	translations := unsafe.Slice((*langAndCodePage)(unsafe.Pointer(valuePtr)), translationCount)
	if len(translations) == 0 {
		gorillalog.Warn("Unable to get additional metadata:", path)
		return finalMetadata
	}
	translation := translations[0]

	productKey := fmt.Sprintf("\\StringFileInfo\\%04x%04x\\ProductName", translation.Lang, translation.CodePage)
	valuePtr, _, ok = verQueryValue(rawMetadata, productKey)
	if !ok || valuePtr == 0 {
		gorillalog.Info("Unable to get product name from metadata:", path)
		return finalMetadata
	}
	finalMetadata.productName = windows.UTF16PtrToString((*uint16)(unsafe.Pointer(valuePtr)))
	return finalMetadata
}
