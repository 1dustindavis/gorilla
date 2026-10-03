package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/admin"
	"github.com/1dustindavis/gorilla/pkg/config"
)

func TestAdminBuildDispatchBypassesClientConfigAndElevation(t *testing.T) {
	originalConfigGet := configGetFunc
	originalBuild := adminBuildCatalogsFunc
	originalAdminCheck := adminCheckFunc
	t.Cleanup(func() {
		configGetFunc = originalConfigGet
		adminBuildCatalogsFunc = originalBuild
		adminCheckFunc = originalAdminCheck
	})

	configGetFunc = func() config.Configuration {
		t.Fatalf("config.Get must not be called for admin commands")
		return config.Configuration{}
	}
	adminCheckFunc = func() (bool, error) {
		t.Fatalf("Windows administrative privilege check must not be called for admin commands")
		return false, nil
	}
	adminBuildCatalogsFunc = func(repoPath string) (admin.BuildResult, error) {
		if repoPath != "repo-path" {
			t.Fatalf("repoPath = %q, want repo-path", repoPath)
		}
		return admin.BuildResult{Records: 42, Catalogs: 3}, nil
	}

	if err := run([]string{"gorilla", "admin", "build", "--repo", "repo-path"}); err != nil {
		t.Fatalf("run() error = %v", err)
	}
}

func TestRunAdminBuildDefaultsToCurrentWorkingDirectory(t *testing.T) {
	originalBuild := adminBuildCatalogsFunc
	originalGetwd := adminGetwdFunc
	t.Cleanup(func() {
		adminBuildCatalogsFunc = originalBuild
		adminGetwdFunc = originalGetwd
	})

	adminGetwdFunc = func() (string, error) { return "repo-path", nil }
	adminBuildCatalogsFunc = func(repoPath string) (admin.BuildResult, error) {
		if repoPath != "repo-path" {
			t.Fatalf("repoPath = %q, want repo-path", repoPath)
		}
		return admin.BuildResult{Records: 2, Catalogs: 1}, nil
	}

	var stdout bytes.Buffer
	if err := runAdmin([]string{"build"}, &stdout); err != nil {
		t.Fatalf("runAdmin() error = %v", err)
	}
	for _, want := range []string{
		"Building catalogs from repo-path",
		"Loaded 2 package-info records",
		"Generated 1 catalogs",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want substring %q", stdout.String(), want)
		}
	}
}
