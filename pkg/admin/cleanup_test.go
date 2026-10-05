package admin

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeRepositoryManifest(t *testing.T, repo, relative, contents string) string {
	t.Helper()
	path := filepath.Join(repo, "manifests", relative)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeRepositoryAsset(t *testing.T, repo, relative, contents string) string {
	t.Helper()
	path := filepath.Join(repo, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func cleanupItem(t *testing.T, plan CleanupPlan, catalogName, itemName string) CleanupItem {
	t.Helper()
	for _, item := range plan.Items {
		if item.Catalog == catalogName && item.ItemName == itemName {
			return item
		}
	}
	t.Fatalf("cleanup item %s/%s not found", catalogName, itemName)
	return CleanupItem{}
}

func dispositions(item CleanupItem) []VersionDisposition {
	out := make([]VersionDisposition, len(item.Versions))
	for i, version := range item.Versions {
		out[i] = version.Disposition
	}
	return out
}

func TestPlanCleanupCombinesAllManifestRootsAndExpandsCurrentDependencies(t *testing.T) {
	repo := t.TempDir()
	writeRepositoryManifest(t, repo, "primary.yaml", `
name: primary
managed_installs:
  - AppA
optional_installs:
  - OptionalRoot
managed_updates:
  - UpdateRoot
managed_uninstalls:
  - UninstallRoot
`)
	writeRepositoryManifest(t, repo, filepath.Join("nested", "secondary.yml"), `
name: secondary
managed_installs:
  - OnlyInSecondManifest
  - SharedName
`)
	if err := os.WriteFile(filepath.Join(repo, "manifests", "ignored.txt"), []byte("ignored"), 0644); err != nil {
		t.Fatal(err)
	}

	writePackageInfo(t, repo, "app-a-2.yaml", `
item_name: AppA
catalog: base
version: 2.0
dependencies:
  - AppB
`)
	writePackageInfo(t, repo, "app-a-1.yaml", `
item_name: AppA
catalog: base
version: 1.0
dependencies:
  - OldDependency
`)
	writePackageInfo(t, repo, "app-b.yaml", `
item_name: AppB
catalog: base
version: 1.0
dependencies:
  - RuntimeC
`)
	writePackageInfo(t, repo, "runtime-c.yaml", `
item_name: RuntimeC
catalog: base
version: 1.0
dependencies:
  - AppA
`)
	writePackageInfo(t, repo, "old-dependency.yaml", "item_name: OldDependency\ncatalog: base\nversion: 1.0\n")
	writePackageInfo(t, repo, "optional.yaml", "item_name: OptionalRoot\ncatalog: base\nversion: 1.0\n")
	writePackageInfo(t, repo, "update.yaml", "item_name: UpdateRoot\ncatalog: base\nversion: 1.0\n")
	writePackageInfo(t, repo, "uninstall.yaml", "item_name: UninstallRoot\ncatalog: base\nversion: 1.0\n")
	writePackageInfo(t, repo, "second.yaml", "item_name: OnlyInSecondManifest\ncatalog: base\nversion: 1.0\n")
	writePackageInfo(t, repo, "shared-base.yaml", "item_name: SharedName\ncatalog: base\nversion: 1.0\n")
	writePackageInfo(t, repo, "shared-optional.yaml", "item_name: SharedName\ncatalog: optional\nversion: 9.0\n")

	plan, err := PlanCleanup(repo, CleanupOptions{Keep: 3})
	if err != nil {
		t.Fatalf("PlanCleanup() error = %v", err)
	}

	for _, identity := range [][2]string{
		{"base", "AppA"},
		{"base", "AppB"},
		{"base", "RuntimeC"},
		{"base", "OptionalRoot"},
		{"base", "UpdateRoot"},
		{"base", "UninstallRoot"},
		{"base", "OnlyInSecondManifest"},
		{"base", "SharedName"},
		{"optional", "SharedName"},
	} {
		if item := cleanupItem(t, plan, identity[0], identity[1]); !item.Live {
			t.Errorf("%s/%s should be live", identity[0], identity[1])
		}
	}
	if item := cleanupItem(t, plan, "base", "OldDependency"); item.Live {
		t.Fatalf("historical dependency OldDependency should be abandoned")
	}
	if got := dispositions(cleanupItem(t, plan, "base", "OldDependency")); !reflect.DeepEqual(got, []VersionDisposition{VersionAbandoned}) {
		t.Fatalf("OldDependency dispositions = %v, want abandoned", got)
	}
}

func TestPlanCleanupClassifiesRetentionAssetsAndMissingReferences(t *testing.T) {
	repo := t.TempDir()
	writeRepositoryManifest(t, repo, "main.yaml", "name: main\nmanaged_installs:\n  - App\n")

	writePackageInfo(t, repo, "app-5.yaml", `
item_name: App
catalog: base
version: 5.0
installer:
  location: packages/app/app-5.exe
icon: icons/app.png
`)
	writePackageInfo(t, repo, "app-4.yaml", "item_name: App\ncatalog: base\nversion: 4.0\n")
	writePackageInfo(t, repo, "app-3.yaml", "item_name: App\ncatalog: base\nversion: 3.0\n")
	writePackageInfo(t, repo, "app-2.yaml", "item_name: App\ncatalog: base\nversion: 2.0\n")
	writePackageInfo(t, repo, "app-1.yaml", `
item_name: App
catalog: base
version: 1.0
installer:
  location: packages/app/app-1.exe
`)
	writePackageInfo(t, repo, "legacy-3.yaml", `
item_name: Legacy
catalog: base
version: 3.0
installer:
  location: packages/legacy/legacy.exe
`)
	writePackageInfo(t, repo, "legacy-2.yaml", "item_name: Legacy\ncatalog: base\nversion: 2.0\n")

	writeRepositoryAsset(t, repo, "packages/app/app-5.exe", "current")
	writeRepositoryAsset(t, repo, "packages/app/app-1.exe", "superseded")
	writeRepositoryAsset(t, repo, "packages/legacy/legacy.exe", "abandoned")
	writeRepositoryAsset(t, repo, "packages/orphan.bin", "orphan")
	writeRepositoryAsset(t, repo, "packages/.gitkeep", "")
	writeRepositoryAsset(t, repo, "icons/.gitkeep", "")

	plan, err := PlanCleanup(repo, CleanupOptions{Keep: 3})
	if err != nil {
		t.Fatalf("PlanCleanup() error = %v", err)
	}
	if plan.Keep != 3 {
		t.Fatalf("plan.Keep = %d, want 3", plan.Keep)
	}

	app := cleanupItem(t, plan, "base", "App")
	if !app.Live {
		t.Fatalf("App should be live")
	}
	wantApp := []VersionDisposition{
		VersionCurrent,
		VersionRetained,
		VersionRetained,
		VersionSuperseded,
		VersionSuperseded,
	}
	if got := dispositions(app); !reflect.DeepEqual(got, wantApp) {
		t.Fatalf("App dispositions = %v, want %v", got, wantApp)
	}

	legacy := cleanupItem(t, plan, "base", "Legacy")
	if legacy.Live {
		t.Fatalf("Legacy should be abandoned")
	}
	if got := dispositions(legacy); !reflect.DeepEqual(got, []VersionDisposition{VersionAbandoned, VersionAbandoned}) {
		t.Fatalf("Legacy dispositions = %v, want all abandoned", got)
	}

	wantAbandonedFiles := []string{
		"packages/app/app-1.exe",
		"packages/legacy/legacy.exe",
		"packages/orphan.bin",
	}
	if !reflect.DeepEqual(plan.AbandonedFiles, wantAbandonedFiles) {
		t.Fatalf("AbandonedFiles = %#v, want %#v", plan.AbandonedFiles, wantAbandonedFiles)
	}
	for _, path := range plan.AbandonedFiles {
		if strings.HasSuffix(path, ".gitkeep") {
			t.Fatalf("marker file was incorrectly abandoned: %s", path)
		}
	}

	if len(plan.MissingAssets) != 1 || plan.MissingAssets[0].Path != "icons/app.png" {
		t.Fatalf("MissingAssets = %#v, want icons/app.png", plan.MissingAssets)
	}
	if got := plan.MissingAssets[0].ReferencedBy; len(got) != 1 || got[0].ItemName != "App" || got[0].Version != "5.0" {
		t.Fatalf("missing asset references = %#v", got)
	}
}

func TestPlanCleanupIgnoresInlinePowerShellScriptsAsAssets(t *testing.T) {
	repo := t.TempDir()
	writeRepositoryManifest(t, repo, "main.yaml", "managed_installs:\n  - App\n")
	writePackageInfo(t, repo, "app.yaml", `
item_name: App
catalog: base
version: 1.0
check:
  script: |
    $path = "C:\Program Files\Example\example.exe"
    Test-Path $path
preinstall_script: |
  $temp = "C:\Windows\Temp\gorilla"
  New-Item -ItemType Directory -Force -Path $temp
postinstall_script: |
  Write-Host "Installed to C:\Program Files\Example"
`)

	plan, err := PlanCleanup(repo, CleanupOptions{Keep: 3})
	if err != nil {
		t.Fatalf("PlanCleanup() error = %v", err)
	}
	if len(plan.MissingAssets) != 0 {
		t.Fatalf("MissingAssets = %#v, want none", plan.MissingAssets)
	}
	if len(plan.AbandonedFiles) != 0 {
		t.Fatalf("AbandonedFiles = %#v, want none", plan.AbandonedFiles)
	}
}

func TestPlanCleanupKeepOneRetainsOnlyCurrent(t *testing.T) {
	repo := t.TempDir()
	writeRepositoryManifest(t, repo, "main.yaml", "managed_installs:\n  - App\n")
	for version := 1; version <= 5; version++ {
		writePackageInfo(t, repo, fmt.Sprintf("app-%d.yaml", version), fmt.Sprintf("item_name: App\ncatalog: base\nversion: %d.0\n", version))
	}

	plan, err := PlanCleanup(repo, CleanupOptions{Keep: 1})
	if err != nil {
		t.Fatalf("PlanCleanup() error = %v", err)
	}
	want := []VersionDisposition{VersionCurrent, VersionSuperseded, VersionSuperseded, VersionSuperseded, VersionSuperseded}
	if got := dispositions(cleanupItem(t, plan, "base", "App")); !reflect.DeepEqual(got, want) {
		t.Fatalf("dispositions = %v, want %v", got, want)
	}
}

func TestPlanCleanupDefaultsKeepToThree(t *testing.T) {
	repo := t.TempDir()
	writeRepositoryManifest(t, repo, "main.yaml", "managed_installs:\n  - App\n")
	writePackageInfo(t, repo, "app.yaml", "item_name: App\ncatalog: base\nversion: 1.0\n")

	plan, err := PlanCleanup(repo, CleanupOptions{})
	if err != nil {
		t.Fatalf("PlanCleanup() error = %v", err)
	}
	if plan.Keep != DefaultKeepVersions {
		t.Fatalf("plan.Keep = %d, want %d", plan.Keep, DefaultKeepVersions)
	}
}

func TestPlanCleanupRejectsUnsafeAssetPaths(t *testing.T) {
	for _, unsafe := range []string{
		"../../secret.txt",
		"/etc/passwd",
		`C:\Windows\system32\foo.dll`,
		`\\server\share\foo.dll`,
		"https://example.test/setup.exe",
	} {
		t.Run(strings.ReplaceAll(unsafe, "/", "_"), func(t *testing.T) {
			repo := t.TempDir()
			writeRepositoryManifest(t, repo, "main.yaml", "managed_installs:\n  - App\n")
			writePackageInfo(t, repo, "app.yaml", fmt.Sprintf("item_name: App\ncatalog: base\nversion: 1.0\nicon: '%s'\n", unsafe))

			_, err := PlanCleanup(repo, CleanupOptions{Keep: 3})
			if err == nil {
				t.Fatalf("PlanCleanup() accepted unsafe path %q", unsafe)
			}
			if !strings.Contains(err.Error(), "unsafe repository asset path") {
				t.Fatalf("error = %q, want unsafe repository asset path", err)
			}
		})
	}
}

func TestPlanCleanupFailsWithoutCompleteManifestSet(t *testing.T) {
	t.Run("missing manifests directory", func(t *testing.T) {
		repo := t.TempDir()
		writePackageInfo(t, repo, "app.yaml", "item_name: App\ncatalog: base\nversion: 1.0\n")
		_, err := PlanCleanup(repo, CleanupOptions{Keep: 3})
		if err == nil || !strings.Contains(err.Error(), "manifests directory does not exist") {
			t.Fatalf("PlanCleanup() error = %v, want missing manifests directory", err)
		}
	})

	t.Run("malformed manifest", func(t *testing.T) {
		repo := t.TempDir()
		writePackageInfo(t, repo, "app.yaml", "item_name: App\ncatalog: base\nversion: 1.0\n")
		writeRepositoryManifest(t, repo, "broken.yaml", "managed_installs: [unterminated\n")
		_, err := PlanCleanup(repo, CleanupOptions{Keep: 3})
		if err == nil || !strings.Contains(err.Error(), "parse repository manifest manifests/broken.yaml") {
			t.Fatalf("PlanCleanup() error = %v, want malformed manifest error", err)
		}
	})
}

func TestPlanCleanupDoesNotModifyRepositoryFiles(t *testing.T) {
	repo := t.TempDir()
	writeRepositoryManifest(t, repo, "main.yaml", "managed_installs:\n  - App\n")
	packageInfoPath := writePackageInfo(t, repo, "app-old.yaml", "item_name: App\ncatalog: base\nversion: 1.0\n")
	writePackageInfo(t, repo, "app-current.yaml", "item_name: App\ncatalog: base\nversion: 2.0\n")
	candidatePath := writeRepositoryAsset(t, repo, "packages/unused.exe", "do not delete")

	beforePackageInfo, err := os.ReadFile(packageInfoPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeCandidate, err := os.ReadFile(candidatePath)
	if err != nil {
		t.Fatal(err)
	}

	plan, err := PlanCleanup(repo, CleanupOptions{Keep: 1})
	if err != nil {
		t.Fatalf("PlanCleanup() error = %v", err)
	}
	if !reflect.DeepEqual(plan.AbandonedFiles, []string{"packages/unused.exe"}) {
		t.Fatalf("AbandonedFiles = %#v", plan.AbandonedFiles)
	}

	afterPackageInfo, err := os.ReadFile(packageInfoPath)
	if err != nil {
		t.Fatalf("package-info candidate was removed: %v", err)
	}
	afterCandidate, err := os.ReadFile(candidatePath)
	if err != nil {
		t.Fatalf("asset candidate was removed: %v", err)
	}
	if !reflect.DeepEqual(afterPackageInfo, beforePackageInfo) || !reflect.DeepEqual(afterCandidate, beforeCandidate) {
		t.Fatalf("PlanCleanup modified repository contents")
	}
}
