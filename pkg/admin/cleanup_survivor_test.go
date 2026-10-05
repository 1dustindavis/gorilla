package admin

import (
	"reflect"
	"testing"
)

func TestPlanCleanupIgnoresUnsafeAssetPathsInObsoleteRecords(t *testing.T) {
	repo := t.TempDir()
	writeRepositoryManifest(t, repo, "main.yaml", "managed_installs:\n  - App\n")

	writePackageInfo(t, repo, "app-current.yaml", `
item_name: App
catalog: base
version: 2.0
`)
	writePackageInfo(t, repo, "app-superseded.yaml", `
item_name: App
catalog: base
version: 1.0
icon: C:\Windows\legacy.ico
`)
	writePackageInfo(t, repo, "abandoned.yaml", `
item_name: Abandoned
catalog: base
version: 1.0
installer:
  location: ../../legacy/setup.exe
`)

	plan, err := PlanCleanup(repo, CleanupOptions{Keep: 1})
	if err != nil {
		t.Fatalf("PlanCleanup() error = %v", err)
	}

	if got := dispositions(cleanupItem(t, plan, "base", "App")); !reflect.DeepEqual(got, []VersionDisposition{VersionCurrent, VersionSuperseded}) {
		t.Fatalf("App dispositions = %v, want current/superseded", got)
	}
	if got := dispositions(cleanupItem(t, plan, "base", "Abandoned")); !reflect.DeepEqual(got, []VersionDisposition{VersionAbandoned}) {
		t.Fatalf("Abandoned dispositions = %v, want abandoned", got)
	}
	if len(plan.MissingAssets) != 0 {
		t.Fatalf("MissingAssets = %#v, want none", plan.MissingAssets)
	}
}
