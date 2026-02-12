//go:build !windows

package main

import (
	"errors"

	"github.com/1dustindavis/gorilla/pkg/config"
)

func sendServiceCommand(_ config.Configuration, _ string) error {
	return errors.New("service commands are only supported on Windows")
}
