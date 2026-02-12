//go:build windows
// +build windows

package status

import (
	"testing"
	"unsafe"
)

func resetMetadataHooks() {
	getFileVersionInfoSizeFunc = getFileVersionInfoSize
	getFileVersionInfoFunc = getFileVersionInfo
	verQueryValueFunc = verQueryValue
}

func TestGetFileMetadataWindows(t *testing.T) {
	resetMetadataHooks()
	defer resetMetadataHooks()

	md := GetFileMetadata("testdata/test.exe")
	if md.versionString != "3.2.0.1" {
		t.Fatalf("unexpected versionString: got %q want %q", md.versionString, "3.2.0.1")
	}
	if md.versionMajor != 3 || md.versionMinor != 2 || md.versionPatch != 0 || md.versionBuild != 1 {
		t.Fatalf("unexpected version parts: got %d.%d.%d.%d", md.versionMajor, md.versionMinor, md.versionPatch, md.versionBuild)
	}
	if md.productName == "" {
		t.Fatalf("expected non-empty productName")
	}
}

func TestGetFileMetadataRootQueryFailure(t *testing.T) {
	resetMetadataHooks()
	defer resetMetadataHooks()

	getFileVersionInfoSizeFunc = func(_ string) uint32 { return 16 }
	getFileVersionInfoFunc = func(_ string, _ []byte) bool { return true }
	verQueryValueFunc = func(_ []byte, subBlock string) (uintptr, uint32, bool) {
		if subBlock == "\\" {
			return 0, 0, false
		}
		return 0, 0, false
	}

	md := GetFileMetadata("testdata/test.exe")
	if md.versionString != "" {
		t.Fatalf("expected empty versionString on root query failure, got %q", md.versionString)
	}
}

func TestGetFileMetadataInvalidSignature(t *testing.T) {
	resetMetadataHooks()
	defer resetMetadataHooks()

	getFileVersionInfoSizeFunc = func(_ string) uint32 { return 16 }
	getFileVersionInfoFunc = func(_ string, _ []byte) bool { return true }

	fixed := vsFixedFileInfo{
		Signature:     0,
		FileVersionMS: uint32(3<<16) | 2,
		FileVersionLS: uint32(0<<16) | 1,
	}
	verQueryValueFunc = func(_ []byte, subBlock string) (uintptr, uint32, bool) {
		if subBlock == "\\" {
			return uintptr(unsafe.Pointer(&fixed)), uint32(unsafe.Sizeof(fixed)), true
		}
		return 0, 0, false
	}

	md := GetFileMetadata("testdata/test.exe")
	if md.versionString != "" {
		t.Fatalf("expected empty versionString on invalid signature, got %q", md.versionString)
	}
}

func TestGetFileMetadataTranslationFailure(t *testing.T) {
	resetMetadataHooks()
	defer resetMetadataHooks()

	getFileVersionInfoSizeFunc = func(_ string) uint32 { return 16 }
	getFileVersionInfoFunc = func(_ string, _ []byte) bool { return true }

	fixed := vsFixedFileInfo{
		Signature:     0xFEEF04BD,
		FileVersionMS: uint32(3<<16) | 2,
		FileVersionLS: uint32(0<<16) | 1,
	}
	verQueryValueFunc = func(_ []byte, subBlock string) (uintptr, uint32, bool) {
		if subBlock == "\\" {
			return uintptr(unsafe.Pointer(&fixed)), uint32(unsafe.Sizeof(fixed)), true
		}
		if subBlock == "\\VarFileInfo\\Translation" {
			return 0, 0, false
		}
		return 0, 0, false
	}

	md := GetFileMetadata("testdata/test.exe")
	if md.versionString != "3.2.0.1" {
		t.Fatalf("expected parsed version before translation failure, got %q", md.versionString)
	}
	if md.productName != "" {
		t.Fatalf("expected empty productName when translation fails, got %q", md.productName)
	}
}

func TestGetFileMetadataProductNameMissing(t *testing.T) {
	resetMetadataHooks()
	defer resetMetadataHooks()

	getFileVersionInfoSizeFunc = func(_ string) uint32 { return 16 }
	getFileVersionInfoFunc = func(_ string, _ []byte) bool { return true }

	fixed := vsFixedFileInfo{
		Signature:     0xFEEF04BD,
		FileVersionMS: uint32(3<<16) | 2,
		FileVersionLS: uint32(0<<16) | 1,
	}
	translation := langAndCodePage{Lang: 0x0409, CodePage: 0x04E4}
	verQueryValueFunc = func(_ []byte, subBlock string) (uintptr, uint32, bool) {
		if subBlock == "\\" {
			return uintptr(unsafe.Pointer(&fixed)), uint32(unsafe.Sizeof(fixed)), true
		}
		if subBlock == "\\VarFileInfo\\Translation" {
			return uintptr(unsafe.Pointer(&translation)), uint32(unsafe.Sizeof(translation)), true
		}
		return 0, 0, false
	}

	md := GetFileMetadata("testdata/test.exe")
	if md.versionString != "3.2.0.1" {
		t.Fatalf("expected parsed version when product name lookup fails, got %q", md.versionString)
	}
	if md.productName != "" {
		t.Fatalf("expected empty productName when lookup fails, got %q", md.productName)
	}
}
