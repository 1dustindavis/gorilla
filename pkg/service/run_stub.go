//go:build !windows

package service

import (
	"errors"

	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/managedrun"
)

func Run(_ config.Configuration, _ managedrun.RunFunc, _ ...managedrun.ItemRunFunc) error {
	return errors.New("service mode is only supported on Windows")
}
