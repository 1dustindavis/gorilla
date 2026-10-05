package admin

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func restoreCleanupApplySeams(t *testing.T) {
	t.Helper()
	originalBuild := cleanupBuildCatalogs
	originalLstat := cleanupLstat
	originalMkdirAll := cleanupMkdirAll
	originalMkdirTemp := cleanupMkdirTemp
	originalReadDir := cleanupReadDir
	originalRemove := cleanupRemove
	originalRemoveAll := cleanupRemoveAll
	originalRename := cleanupRename
	t.Cleanup(func() {
		cleanupBuildCatalogs = originalBuild
		cleanupLstat = originalLstat
		cleanupMkdirAll = originalMkdirAll
		cleanupMkdirTemp = originalMkdirTemp
		cleanupReadDir = originalReadDir
		cleanupRemove = originalRemove
		cleanupRemoveAll = originalRemoveAll
		cleanupRename = originalRename
	})
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be missing, stat error = %v", path, err)
	}
}

func TestApplyCleanupRemovesPlannedCandidatesAndRebuildsCatalogs(t *testing.T) {
	repo := t.TempDir()
	writeRepositoryManifest(t, repo, "main.yaml", "managed_installs:\n  - Chrome\n")

	chrome5 := writePackageInfo(t, repo, "chrome-5.yaml", `
item_name: Chrome
catalog: base
version: 5.0
installer:
  location: packages/chrome/shared.exe
icon: icons/chrome.png
`)
	chrome4 := writePackageInfo(t, repo, "chrome-4.yaml", `
item_name: Chrome
catalog: base
version: 4.0
installer:
  location: packages/chrome/shared.exe
`)
	chrome3 := writePackageInfo(t, repo, "chrome-3.yaml", "item_name: Chrome\ncatalog: base\nversion: 3.0\n")
	chrome2 := writePackageInfo(t, repo, "chrome-2.yaml", `
item_name: Chrome
catalog: base
version: 2.0
installer:
  location: packages/chrome/old.exe
`)
	legacy5 := writePackageInfo(t, repo, "legacy-5.yaml", `
item_name: LegacyVPN
catalog: base
version: 5.0
installer:
  location: packages/legacy/legacy.exe
`)
	legacy4 := writePackageInfo(t, repo, "legacy-4.yaml", "item_name: LegacyVPN\ncatalog: base\nversion: 4.0\n")

	shared := writeRepositoryAsset(t, repo, "packages/chrome/shared.exe", "shared")
	old := writeRepositoryAsset(t, repo, "packages/chrome/old.exe", "old")
	legacy := writeRepositoryAsset(t, repo, "packages/legacy/legacy.exe", "legacy")
	orphan := writeRepositoryAsset(t, repo, "packages/orphan/unused.bin", "orphan")
	icon := writeRepositoryAsset(t, repo, "icons/chrome.png", "icon")
	writeRepositoryAsset(t, repo, "packages/keep/.gitkeep", "")
	unrelated := filepath.Join(repo, "README.md")
	if err := os.WriteFile(unrelated, []byte("leave me alone"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := BuildCatalogs(repo); err != nil {
		t.Fatalf("BuildCatalogs() before cleanup error = %v", err)
	}

	plan, err := PlanCleanup(repo, CleanupOptions{Keep: 3})
	if err != nil {
		t.Fatalf("PlanCleanup() error = %v", err)
	}
	result, err := ApplyCleanup(plan)
	if err != nil {
		t.Fatalf("ApplyCleanup() error = %v", err)
	}
	if result.PackageInfoRemoved != 3 || result.AssetsRemoved != 3 || result.CatalogsGenerated != 1 {
		t.Fatalf("ApplyCleanup() result = %#v, want 3 package-info, 3 assets, 1 catalog", result)
	}
	if result.DirectoriesRemoved < 2 {
		t.Fatalf("DirectoriesRemoved = %d, want at least legacy/orphan directories", result.DirectoriesRemoved)
	}

	for _, path := range []string{chrome5, chrome4, chrome3, shared, icon, unrelated} {
		assertPathExists(t, path)
	}
	for _, path := range []string{chrome2, legacy5, legacy4, old, legacy, orphan} {
		assertPathMissing(t, path)
	}
	assertPathExists(t, filepath.Join(repo, "packages"))
	assertPathExists(t, filepath.Join(repo, "icons"))
	assertPathExists(t, filepath.Join(repo, "packages-info"))
	assertPathExists(t, filepath.Join(repo, "packages", "keep", ".gitkeep"))
	assertPathExists(t, filepath.Join(repo, "packages", "keep"))

	catalog := readCatalog(t, repo, "base")
	if _, ok := catalog["LegacyVPN"]; ok {
		t.Fatalf("generated catalog still contains abandoned LegacyVPN: %#v", catalog)
	}
	if got := catalog["Chrome"].Version; got != "5.0" {
		t.Fatalf("generated Chrome version = %q, want 5.0", got)
	}

	staging, err := filepath.Glob(filepath.Join(repo, ".gorilla-cleanup-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(staging) != 0 {
		t.Fatalf("staging directories remain after successful cleanup: %#v", staging)
	}
}

func TestApplyCleanupAssetOnlyStillRebuildsCatalogs(t *testing.T) {
	repo := t.TempDir()
	writeRepositoryManifest(t, repo, "main.yaml", "managed_installs:\n  - App\n")
	writePackageInfo(t, repo, "app.yaml", "item_name: App\ncatalog: base\nversion: 1.0\nicon: icons/missing.png\n")
	orphan := writeRepositoryAsset(t, repo, "packages/orphan.exe", "orphan")

	plan, err := PlanCleanup(repo, CleanupOptions{Keep: 3})
	if err != nil {
		t.Fatalf("PlanCleanup() error = %v", err)
	}
	if len(plan.MissingAssets) != 1 || plan.MissingAssets[0].Path != "icons/missing.png" {
		t.Fatalf("MissingAssets = %#v, want icons/missing.png", plan.MissingAssets)
	}
	result, err := ApplyCleanup(plan)
	if err != nil {
		t.Fatalf("ApplyCleanup() error = %v", err)
	}
	if result.PackageInfoRemoved != 0 || result.AssetsRemoved != 1 || result.CatalogsGenerated != 1 {
		t.Fatalf("ApplyCleanup() result = %#v", result)
	}
	assertPathMissing(t, orphan)
	if got := readCatalog(t, repo, "base")["App"].Version; got != "1.0" {
		t.Fatalf("catalog App version = %q, want 1.0", got)
	}
}

func TestApplyCleanupNoOpDoesNotCreateStagingOrBuildCatalogs(t *testing.T) {
	restoreCleanupApplySeams(t)
	repo := t.TempDir()
	writeRepositoryManifest(t, repo, "main.yaml", "managed_installs:\n  - App\n")
	writePackageInfo(t, repo, "app.yaml", "item_name: App\ncatalog: base\nversion: 1.0\n")
	plan, err := PlanCleanup(repo, CleanupOptions{Keep: 3})
	if err != nil {
		t.Fatalf("PlanCleanup() error = %v", err)
	}

	cleanupMkdirTemp = func(string, string) (string, error) {
		t.Fatalf("no-op cleanup must not create staging")
		return "", nil
	}
	cleanupBuildCatalogs = func(string) (BuildResult, error) {
		t.Fatalf("no-op cleanup must not rebuild catalogs")
		return BuildResult{}, nil
	}

	result, err := ApplyCleanup(plan)
	if err != nil {
		t.Fatalf("ApplyCleanup() error = %v", err)
	}
	if result != (CleanupResult{}) {
		t.Fatalf("ApplyCleanup() result = %#v, want zero result", result)
	}
}

func TestApplyCleanupRejectsUnsafeManualCandidatesBeforeMutation(t *testing.T) {
	tests := []struct {
		name string
		path string
		kind cleanupCandidateKind
	}{
		{name: "traversal", path: "../../outside", kind: cleanupPackageInfo},
		{name: "unix absolute", path: "/etc/passwd", kind: cleanupPackageInfo},
		{name: "windows absolute", path: `C:\Windows\system32\foo.dll`, kind: cleanupAsset},
		{name: "manifest", path: "manifests/foo.yaml", kind: cleanupPackageInfo},
		{name: "readme", path: "README.md", kind: cleanupAsset},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restoreCleanupApplySeams(t)
			repo := t.TempDir()
			called := false
			cleanupMkdirTemp = func(string, string) (string, error) {
				called = true
				return "", errors.New("must not be called")
			}
			plan := CleanupPlan{RepoPath: repo}
			if tt.kind == cleanupPackageInfo {
				plan.Items = []CleanupItem{{Versions: []CleanupVersion{{PackageInfo: tt.path, Disposition: VersionSuperseded}}}}
			} else {
				plan.AbandonedFiles = []string{tt.path}
			}
			if _, err := ApplyCleanup(plan); err == nil {
				t.Fatalf("ApplyCleanup() accepted unsafe path %q", tt.path)
			}
			if called {
				t.Fatalf("staging created for unsafe path %q", tt.path)
			}
		})
	}
}

func TestApplyCleanupRejectsSymlinkCandidate(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "packages"), 0755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(repo, "target.bin")
	if err := os.WriteFile(target, []byte("target"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(repo, "packages", "link.bin")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	plan := CleanupPlan{RepoPath: repo, AbandonedFiles: []string{"packages/link.bin"}}
	if _, err := ApplyCleanup(plan); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("ApplyCleanup() error = %v, want symlink validation failure", err)
	}
	assertPathExists(t, target)
}

func TestApplyCleanupFailsValidationWhenCandidateDisappears(t *testing.T) {
	repo := t.TempDir()
	writeRepositoryManifest(t, repo, "main.yaml", "managed_installs:\n  - App\n")
	writePackageInfo(t, repo, "app-3.yaml", "item_name: App\ncatalog: base\nversion: 3.0\n")
	old2 := writePackageInfo(t, repo, "app-2.yaml", "item_name: App\ncatalog: base\nversion: 2.0\n")
	old1 := writePackageInfo(t, repo, "app-1.yaml", "item_name: App\ncatalog: base\nversion: 1.0\n")
	plan, err := PlanCleanup(repo, CleanupOptions{Keep: 1})
	if err != nil {
		t.Fatalf("PlanCleanup() error = %v", err)
	}
	if err := os.Remove(old2); err != nil {
		t.Fatal(err)
	}

	if _, err := ApplyCleanup(plan); err == nil {
		t.Fatalf("ApplyCleanup() expected stale candidate validation error")
	}
	assertPathExists(t, old1)
}

func TestApplyCleanupStagingFailureRollsBackMovedCandidates(t *testing.T) {
	restoreCleanupApplySeams(t)
	repo := t.TempDir()
	writeRepositoryManifest(t, repo, "main.yaml", "managed_installs:\n  - App\n")
	writePackageInfo(t, repo, "app-3.yaml", "item_name: App\ncatalog: base\nversion: 3.0\n")
	old2 := writePackageInfo(t, repo, "app-2.yaml", "item_name: App\ncatalog: base\nversion: 2.0\n")
	old1 := writePackageInfo(t, repo, "app-1.yaml", "item_name: App\ncatalog: base\nversion: 1.0\n")
	plan, err := PlanCleanup(repo, CleanupOptions{Keep: 1})
	if err != nil {
		t.Fatalf("PlanCleanup() error = %v", err)
	}

	originalRename := cleanupRename
	stageMoves := 0
	cleanupRename = func(oldPath, newPath string) error {
		if strings.Contains(newPath, ".gorilla-cleanup-") {
			stageMoves++
			if stageMoves == 2 {
				return errors.New("simulated staging failure")
			}
		}
		return originalRename(oldPath, newPath)
	}
	cleanupBuildCatalogs = func(string) (BuildResult, error) {
		t.Fatalf("catalog build must not run after staging failure")
		return BuildResult{}, nil
	}

	if _, err := ApplyCleanup(plan); err == nil || !strings.Contains(err.Error(), "simulated staging failure") {
		t.Fatalf("ApplyCleanup() error = %v, want staging failure", err)
	}
	assertPathExists(t, old2)
	assertPathExists(t, old1)
}

func TestApplyCleanupCatalogFailureRestoresStagedFiles(t *testing.T) {
	restoreCleanupApplySeams(t)
	repo := t.TempDir()
	writeRepositoryManifest(t, repo, "main.yaml", "managed_installs:\n  - App\n")
	writePackageInfo(t, repo, "app-2.yaml", "item_name: App\ncatalog: base\nversion: 2.0\n")
	old := writePackageInfo(t, repo, "app-1.yaml", "item_name: App\ncatalog: base\nversion: 1.0\n")
	asset := writeRepositoryAsset(t, repo, "packages/orphan.bin", "orphan")
	plan, err := PlanCleanup(repo, CleanupOptions{Keep: 1})
	if err != nil {
		t.Fatalf("PlanCleanup() error = %v", err)
	}

	cleanupBuildCatalogs = func(string) (BuildResult, error) {
		return BuildResult{}, errors.New("simulated catalog failure")
	}
	if _, err := ApplyCleanup(plan); err == nil || !strings.Contains(err.Error(), "simulated catalog failure") {
		t.Fatalf("ApplyCleanup() error = %v, want catalog failure", err)
	}
	assertPathExists(t, old)
	assertPathExists(t, asset)
	staging, err := filepath.Glob(filepath.Join(repo, ".gorilla-cleanup-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(staging) != 0 {
		t.Fatalf("staging remains after successful rollback: %#v", staging)
	}
}

func TestApplyCleanupFinalStagingDeletionFailureDoesNotRestoreSources(t *testing.T) {
	restoreCleanupApplySeams(t)
	repo := t.TempDir()
	writeRepositoryManifest(t, repo, "main.yaml", "managed_installs:\n  - App\n")
	writePackageInfo(t, repo, "app-2.yaml", "item_name: App\ncatalog: base\nversion: 2.0\n")
	old := writePackageInfo(t, repo, "app-1.yaml", "item_name: App\ncatalog: base\nversion: 1.0\n")
	if _, err := BuildCatalogs(repo); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanCleanup(repo, CleanupOptions{Keep: 1})
	if err != nil {
		t.Fatalf("PlanCleanup() error = %v", err)
	}

	originalRemoveAll := cleanupRemoveAll
	cleanupRemoveAll = func(path string) error {
		if strings.Contains(filepath.Base(path), ".gorilla-cleanup-") {
			return errors.New("simulated staging deletion failure")
		}
		return originalRemoveAll(path)
	}
	result, err := ApplyCleanup(plan)
	if err == nil || !strings.Contains(err.Error(), "cleanup was applied") {
		t.Fatalf("ApplyCleanup() error = %v, want committed finalization failure", err)
	}
	if result.PackageInfoRemoved != 1 {
		t.Fatalf("ApplyCleanup() result = %#v, want one removed package-info", result)
	}
	assertPathMissing(t, old)
	if got := readCatalog(t, repo, "base")["App"].Version; got != "2.0" {
		t.Fatalf("catalog App version = %q, want 2.0", got)
	}
	staging, globErr := filepath.Glob(filepath.Join(repo, ".gorilla-cleanup-*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(staging) != 1 {
		t.Fatalf("staging directories = %#v, want one residual directory", staging)
	}
}

func TestApplyCleanupCommittedCatalogFinalizationErrorDoesNotRestoreSources(t *testing.T) {
	restoreCleanupApplySeams(t)
	repo := t.TempDir()
	writeRepositoryManifest(t, repo, "main.yaml", "managed_installs:\n  - App\n")
	writePackageInfo(t, repo, "app-2.yaml", "item_name: App\ncatalog: base\nversion: 2.0\n")
	old := writePackageInfo(t, repo, "app-1.yaml", "item_name: App\ncatalog: base\nversion: 1.0\n")
	plan, err := PlanCleanup(repo, CleanupOptions{Keep: 1})
	if err != nil {
		t.Fatalf("PlanCleanup() error = %v", err)
	}

	cleanupBuildCatalogs = func(string) (BuildResult, error) {
		return BuildResult{Records: 1, Catalogs: 1}, &catalogBuildError{
			err:       errors.New("simulated committed catalog finalization failure"),
			committed: true,
		}
	}
	result, err := ApplyCleanup(plan)
	if err == nil || !strings.Contains(err.Error(), "cleanup was applied") {
		t.Fatalf("ApplyCleanup() error = %v, want committed catalog finalization error", err)
	}
	if result.PackageInfoRemoved != 1 || result.CatalogsGenerated != 1 {
		t.Fatalf("ApplyCleanup() result = %#v", result)
	}
	assertPathMissing(t, old)
	staging, globErr := filepath.Glob(filepath.Join(repo, ".gorilla-cleanup-*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(staging) != 0 {
		t.Fatalf("cleanup staging remains after committed catalog finalization error: %#v", staging)
	}
}
