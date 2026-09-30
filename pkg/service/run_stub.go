//go:build !windows

package service

import (
	"errors"

	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/managed"
)

func Run(_ config.Configuration, _ managed.RunFunc, _ ...managed.ItemRunFunc) error {
	return errors.New("service mode is only supported on Windows")
}
