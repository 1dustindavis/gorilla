package admin

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildCatalogsReportsPostCommitFinalizationFailure(t *testing.T) {
	repo := t.TempDir()
	writePackageInfo(t, repo, "app.yaml", "item_name: App\ncatalog: base\nversion: 2.0\n")
	catalogs := filepath.Join(repo, "catalogs")
	if err := os.MkdirAll(catalogs, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(catalogs, "old.yaml"), []byte("Old: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	originalRemoveAll := adminRemoveAll
	t.Cleanup(func() { adminRemoveAll = originalRemoveAll })
	adminRemoveAll = func(path string) error {
		if strings.HasSuffix(path, "-previous") {
			return errors.New("simulated backup cleanup failure")
		}
		return originalRemoveAll(path)
	}

	result, err := BuildCatalogs(repo)
	if err == nil {
		t.Fatalf("BuildCatalogs() expected finalization error")
	}
	if !catalogBuildWasCommitted(err) {
		t.Fatalf("catalogBuildWasCommitted(%v) = false, want true", err)
	}
	if result.Records != 1 || result.Catalogs != 1 {
		t.Fatalf("BuildCatalogs() result = %#v, want committed counts", result)
	}
	if got := readCatalog(t, repo, "base")["App"].Version; got != "2.0" {
		t.Fatalf("committed catalog App version = %q, want 2.0", got)
	}
	if _, statErr := os.Stat(filepath.Join(catalogs, "old.yaml")); !os.IsNotExist(statErr) {
		t.Fatalf("old live catalog still present after commit: %v", statErr)
	}
}
