package main

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/config"
)

func resetMainHooks() {
	adminCheckFunc = adminCheck
	mkdirAllFunc = os.MkdirAll
}

func TestRunAdminCheckError(t *testing.T) {
	resetMainHooks()
	defer resetMainHooks()

	cfg := config.Configuration{CheckOnly: false}
	adminCheckFunc = func() (bool, error) { return false, errors.New("boom") }
	mkdirAllFunc = func(path string, mode os.FileMode) error {
		t.Fatalf("mkdirAllFunc should not be called when admin check errors")
		return nil
	}

	err := run(cfg)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "unable to check if running as admin: boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRequiresAdmin(t *testing.T) {
	resetMainHooks()
	defer resetMainHooks()

	cfg := config.Configuration{CheckOnly: false}
	adminCheckFunc = func() (bool, error) { return false, nil }
	mkdirAllFunc = func(path string, mode os.FileMode) error {
		t.Fatalf("mkdirAllFunc should not be called when admin check fails")
		return nil
	}

	err := run(cfg)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "requires admnisistrative access") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCheckOnlySkipsAdminCheck(t *testing.T) {
	resetMainHooks()
	defer resetMainHooks()

	cfg := config.Configuration{CheckOnly: true, CachePath: "some/cache/path"}
	adminCalled := false
	adminCheckFunc = func() (bool, error) {
		adminCalled = true
		return false, nil
	}
	mkdirAllFunc = func(path string, mode os.FileMode) error { return errors.New("mkdir failed") }

	err := run(cfg)
	if err == nil {
		t.Fatalf("expected error")
	}
	if adminCalled {
		t.Fatalf("adminCheckFunc should not be called in check-only mode")
	}
	if !strings.Contains(err.Error(), "unable to create cache directory: mkdir failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCreateCacheError(t *testing.T) {
	resetMainHooks()
	defer resetMainHooks()

	cfg := config.Configuration{CheckOnly: false, CachePath: "some/cache/path"}
	adminCheckFunc = func() (bool, error) { return true, nil }
	mkdirAllFunc = func(path string, mode os.FileMode) error { return errors.New("mkdir failed") }

	err := run(cfg)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "unable to create cache directory: mkdir failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}
