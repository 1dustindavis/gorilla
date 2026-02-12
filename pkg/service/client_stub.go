//go:build !windows

package service

import (
	"errors"

	"github.com/1dustindavis/gorilla/pkg/config"
)

func sendCommand(_ config.Configuration, _ Command) (CommandResponse, error) {
	return CommandResponse{}, errors.New("service commands are only supported on Windows")
}
