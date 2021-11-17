// Without an OS specific build, go tools will try to include Windows libraries and fail

//go:build !windows
// +build !windows

package main

import (
	"flag"
	"os"
)

func adminCheck() (bool, error) {
	// Skip the check if this is test
	if flag.Lookup("test.v") != nil {
		return false, nil
	}

	if os.Geteuid() == 0 {
		return true, nil
	}

	return false, nil
}
