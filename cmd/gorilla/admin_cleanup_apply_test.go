package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/admin"
)

func TestRunAdminCleanupDryRunNeverApplies(t *testing.T) {
	originalPlan := adminPlanCleanupFunc
	originalApply := adminApplyCleanupFunc
	originalGetwd := adminGetwdFunc
	t.Cleanup(func() {
		adminPlanCleanupFunc = originalPlan
		adminApplyCleanupFunc = originalApply
		adminGetwdFunc = originalGetwd
	})

	adminGetwdFunc = func() (string, error) { return "repo-path", nil }
	adminPlanCleanupFunc = func(repoPath string, options admin.CleanupOptions) (admin.CleanupPlan, error) {
		return admin.CleanupPlan{RepoPath: repoPath, Keep: options.Keep}, nil
	}
	adminApplyCleanupFunc = func(admin.CleanupPlan) (admin.CleanupResult, error) {
		t.Fatalf("apply function called without --apply")
		return admin.CleanupResult{}, nil
	}

	var stdout bytes.Buffer
	if err := runAdmin([]string{"cleanup"}, &stdout); err != nil {
		t.Fatalf("runAdmin() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "Dry run only. No files were changed.") {
		t.Fatalf("stdout = %q, want dry-run message", stdout.String())
	}
}

func TestRunAdminCleanupApplyUsesCreatedPlanAndReportsResult(t *testing.T) {
	originalPlan := adminPlanCleanupFunc
	originalApply := adminApplyCleanupFunc
	t.Cleanup(func() {
		adminPlanCleanupFunc = originalPlan
		adminApplyCleanupFunc = originalApply
	})

	planned := admin.CleanupPlan{
		RepoPath: "repo-path",
		Keep:     2,
		Items: []admin.CleanupItem{{
			Catalog:  "base",
			ItemName: "App",
			Live:     true,
			Versions: []admin.CleanupVersion{{Version: "2.0", Disposition: admin.VersionCurrent}, {Version: "1.0", PackageInfo: "packages-info/app-1.yaml", Disposition: admin.VersionSuperseded}},
		}},
		AbandonedFiles: []string{"packages/old.exe"},
	}
	sequence := []string{}
	adminPlanCleanupFunc = func(repoPath string, options admin.CleanupOptions) (admin.CleanupPlan, error) {
		sequence = append(sequence, "plan")
		if repoPath != "repo-path" || options.Keep != 2 {
			t.Fatalf("planner args = %q, %#v", repoPath, options)
		}
		return planned, nil
	}
	adminApplyCleanupFunc = func(plan admin.CleanupPlan) (admin.CleanupResult, error) {
		sequence = append(sequence, "apply")
		if !reflect.DeepEqual(plan, planned) {
			t.Fatalf("ApplyCleanup plan = %#v, want %#v", plan, planned)
		}
		return admin.CleanupResult{PackageInfoRemoved: 1, AssetsRemoved: 1, DirectoriesRemoved: 2, CatalogsGenerated: 1}, nil
	}

	var stdout bytes.Buffer
	if err := runAdmin([]string{"cleanup", "--repo", "repo-path", "--keep", "2", "--apply"}, &stdout); err != nil {
		t.Fatalf("runAdmin() error = %v", err)
	}
	if !reflect.DeepEqual(sequence, []string{"plan", "apply"}) {
		t.Fatalf("operation sequence = %#v, want plan then apply", sequence)
	}
	for _, want := range []string{
		"SUPERSEDED   1.0",
		"Applying cleanup...",
		"1 package-info files",
		"1 abandoned asset files",
		"2 empty directories",
		"Generated 1 catalogs",
		"Repository cleanup complete.",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want substring %q", stdout.String(), want)
		}
	}
	if strings.Contains(stdout.String(), "Dry run only") {
		t.Fatalf("stdout contains dry-run text after applied cleanup: %q", stdout.String())
	}
}

func TestRunAdminCleanupApplyReportsNoOp(t *testing.T) {
	originalPlan := adminPlanCleanupFunc
	originalApply := adminApplyCleanupFunc
	t.Cleanup(func() {
		adminPlanCleanupFunc = originalPlan
		adminApplyCleanupFunc = originalApply
	})
	adminPlanCleanupFunc = func(repoPath string, options admin.CleanupOptions) (admin.CleanupPlan, error) {
		return admin.CleanupPlan{RepoPath: repoPath, Keep: options.Keep}, nil
	}
	adminApplyCleanupFunc = func(admin.CleanupPlan) (admin.CleanupResult, error) {
		return admin.CleanupResult{}, nil
	}

	var stdout bytes.Buffer
	if err := runAdmin([]string{"cleanup", "--repo", "repo-path", "--apply"}, &stdout); err != nil {
		t.Fatalf("runAdmin() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "Nothing to remove.") || !strings.Contains(stdout.String(), "Repository was not changed.") {
		t.Fatalf("stdout = %q, want no-op result", stdout.String())
	}
}
