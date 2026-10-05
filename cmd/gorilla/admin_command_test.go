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

func TestAdminCleanupDispatchBypassesClientConfigAndElevation(t *testing.T) {
	originalConfigGet := configGetFunc
	originalCleanup := adminPlanCleanupFunc
	originalAdminCheck := adminCheckFunc
	t.Cleanup(func() {
		configGetFunc = originalConfigGet
		adminPlanCleanupFunc = originalCleanup
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
	adminPlanCleanupFunc = func(repoPath string, options admin.CleanupOptions) (admin.CleanupPlan, error) {
		if repoPath != "repo-path" {
			t.Fatalf("repoPath = %q, want repo-path", repoPath)
		}
		if options.Keep != 2 {
			t.Fatalf("Keep = %d, want 2", options.Keep)
		}
		return admin.CleanupPlan{RepoPath: repoPath, Keep: options.Keep}, nil
	}

	if err := run([]string{"gorilla", "admin", "cleanup", "--repo", "repo-path", "--keep", "2"}); err != nil {
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

func TestRunAdminCleanupDefaultsAndReportsDryRun(t *testing.T) {
	originalCleanup := adminPlanCleanupFunc
	originalGetwd := adminGetwdFunc
	t.Cleanup(func() {
		adminPlanCleanupFunc = originalCleanup
		adminGetwdFunc = originalGetwd
	})

	adminGetwdFunc = func() (string, error) { return "repo-path", nil }
	adminPlanCleanupFunc = func(repoPath string, options admin.CleanupOptions) (admin.CleanupPlan, error) {
		if repoPath != "repo-path" {
			t.Fatalf("repoPath = %q, want repo-path", repoPath)
		}
		if options.Keep != admin.DefaultKeepVersions {
			t.Fatalf("Keep = %d, want %d", options.Keep, admin.DefaultKeepVersions)
		}
		return admin.CleanupPlan{
			RepoPath: repoPath,
			Keep:     options.Keep,
			Items: []admin.CleanupItem{
				{
					Catalog:  "base",
					ItemName: "App",
					Live:     true,
					Versions: []admin.CleanupVersion{
						{Version: "3.0", Disposition: admin.VersionCurrent},
						{Version: "2.0", Disposition: admin.VersionRetained},
						{Version: "1.0", Disposition: admin.VersionSuperseded},
					},
				},
				{
					Catalog:  "base",
					ItemName: "Legacy",
					Live:     false,
					Versions: []admin.CleanupVersion{{Version: "1.0", Disposition: admin.VersionAbandoned}},
				},
			},
			AbandonedFiles: []string{"packages/app-1.exe"},
			MissingAssets: []admin.MissingAsset{
				{
					Path: "icons/app.png",
					ReferencedBy: []admin.CleanupAssetReference{
						{Catalog: "base", ItemName: "App", Version: "3.0"},
					},
				},
			},
		}, nil
	}

	var stdout bytes.Buffer
	if err := runAdmin([]string{"cleanup"}, &stdout); err != nil {
		t.Fatalf("runAdmin() error = %v", err)
	}
	for _, want := range []string{
		"Repository cleanup",
		"Repository: repo-path",
		"Retention: 3 versions per live item",
		"CURRENT      3.0",
		"RETAINED     2.0",
		"SUPERSEDED   1.0",
		"ABANDONED    1.0",
		"packages/app-1.exe",
		"icons/app.png",
		"Dry run only. No files were changed.",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want substring %q", stdout.String(), want)
		}
	}
}

func TestRunAdminCleanupRejectsInvalidKeep(t *testing.T) {
	originalCleanup := adminPlanCleanupFunc
	t.Cleanup(func() { adminPlanCleanupFunc = originalCleanup })

	called := false
	adminPlanCleanupFunc = func(repoPath string, options admin.CleanupOptions) (admin.CleanupPlan, error) {
		called = true
		return admin.CleanupPlan{}, nil
	}

	var stdout bytes.Buffer
	err := runAdmin([]string{"cleanup", "--keep", "0"}, &stdout)
	if err == nil || !strings.Contains(err.Error(), "--keep must be at least 1") {
		t.Fatalf("runAdmin() error = %v, want invalid keep", err)
	}
	if called {
		t.Fatalf("cleanup planner was called for invalid --keep")
	}
}
