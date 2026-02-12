//go:build !windows

package main

import (
	"errors"

	"github.com/1dustindavis/gorilla/pkg/config"
)

func runService(_ config.Configuration) error {
	return errors.New("service mode is only supported on Windows")
}
