//go:build !windows

package main

import (
	"errors"

	"github.com/1dustindavis/gorilla/pkg/config"
)

func runServiceAction(_ config.Configuration, _ string) error {
	return errors.New("service install/remove/start/stop is only supported on Windows")
}
