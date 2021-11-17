//go:build windows
// +build windows

package main

import (
	"flag"
	"fmt"

	"golang.org/x/sys/windows"
)

// adminCheck is borrowed from https://github.com/golang/go/issues/28804#issuecomment-438838144
func adminCheck() (bool, error) {
	// Skip the check if this is test
	if flag.Lookup("test.v") != nil {
		return false, nil
	}

	var adminSid *windows.SID

	// Although this looks scary, it is directly copied from the
	// official windows documentation. The Go API for this is a
	// direct wrap around the official C++ API.
	// See https://docs.microsoft.com/en-us/windows/desktop/api/securitybaseapi/nf-securitybaseapi-checktokenmembership
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&adminSid)
	if err != nil {
		return false, fmt.Errorf("SID Error: %v", err)
	}
	defer windows.FreeSid(adminSid)
	// This appears to cast a null pointer so I'm not sure why this
	// works, but this guy says it does and it Works for Me™:
	// https://github.com/golang/go/issues/28804#issuecomment-438838144
	token := windows.Token(0)

	admin, err := token.IsMember(adminSid)
	if err != nil {
		return false, fmt.Errorf("Token Membership Error: %v", err)
	}
	return admin, nil
}
