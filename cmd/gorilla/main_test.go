package main

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/admin"
	"github.com/1dustindavis/gorilla/pkg/config"
)

func resetMainHooks() {
	adminCheckFunc = adminCheck
	mkdirAllFunc = os.MkdirAll
	buildCatalogsFunc = admin.BuildCatalogs
	importItemFunc = admin.ImportItem
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

func TestRunBuildMode(t *testing.T) {
	resetMainHooks()
	defer resetMainHooks()

	cfg := config.Configuration{
		BuildArg:    true,
		RepoPath:    "repo/path",
		CachePath:   "cache/path",
		AppDataPath: t.TempDir(),
	}
	adminCalled := false
	buildCalled := false
	adminCheckFunc = func() (bool, error) {
		adminCalled = true
		return false, nil
	}
	mkdirAllFunc = func(path string, mode os.FileMode) error { return nil }
	buildCatalogsFunc = func(repoPath string) error {
		buildCalled = true
		if repoPath != "repo/path" {
			t.Fatalf("unexpected repoPath: %s", repoPath)
		}
		return nil
	}
	importItemFunc = func(repoPath, itemPath string) error { return nil }

	err := run(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if adminCalled {
		t.Fatalf("adminCheckFunc should not be called in build mode")
	}
	if !buildCalled {
		t.Fatalf("expected buildCatalogsFunc to be called")
	}
}

func TestRunImportModeError(t *testing.T) {
	resetMainHooks()
	defer resetMainHooks()

	cfg := config.Configuration{
		ImportArg:   "x.msi",
		RepoPath:    "repo/path",
		CachePath:   "cache/path",
		AppDataPath: t.TempDir(),
	}
	adminCheckFunc = func() (bool, error) { return true, nil }
	mkdirAllFunc = func(path string, mode os.FileMode) error { return nil }
	buildCatalogsFunc = func(repoPath string) error { return nil }
	importItemFunc = func(repoPath, itemPath string) error {
		if repoPath != "repo/path" || itemPath != "x.msi" {
			t.Fatalf("unexpected args repo=%s item=%s", repoPath, itemPath)
		}
		return errors.New("not implemented")
	}

	err := run(cfg)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "error importing item: not implemented") {
		t.Fatalf("unexpected error: %v", err)
	}
}
