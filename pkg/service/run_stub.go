//go:build !windows

package service

import (
	"errors"

	"github.com/1dustindavis/gorilla/pkg/config"
)

func Run(_ config.Configuration, _ func(config.Configuration) error) error {
	return errors.New("service mode is only supported on Windows")
}
