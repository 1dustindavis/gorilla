//go:build !windows

package service

import (
	"errors"

	"github.com/1dustindavis/gorilla/pkg/config"
)

func RunAction(_ config.Configuration, _ string) error {
	return errors.New("service install/remove/start/stop is only supported on Windows")
}

func ServiceStatus(_ config.Configuration) (string, error) {
	return "", errors.New("service status is only supported on Windows")
}
