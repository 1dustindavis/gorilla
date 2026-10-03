package admin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"go.yaml.in/yaml/v4"
)

func readCatalog(t *testing.T, repo, name string) map[string]catalog.Item {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(repo, "catalogs", name+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var items map[string]catalog.Item
	if err := yaml.Unmarshal(contents, &items); err != nil {
		t.Fatal(err)
	}
	return items
}

func TestBuildCatalogsSelectsNewestVersionDeterministically(t *testing.T) {
	repo := t.TempDir()
	writePackageInfo(t, repo, "z-old.yaml", "item_name: Chrome\ncatalog: base\ndisplay_name: Old Chrome\nversion: 1.0\n")
	writePackageInfo(t, repo, "a-new.yaml", "item_name: Chrome\ncatalog: base\ndisplay_name: New Chrome\nversion: 3.0\n")
	writePackageInfo(t, repo, "m-middle.yaml", "item_name: Chrome\ncatalog: base\ndisplay_name: Middle Chrome\nversion: 2.0\n")
	writePackageInfo(t, repo, "agent.yaml", "item_name: Agent\ncatalog: base\ndisplay_name: Agent\nversion: 7.0\n")
	writePackageInfo(t, repo, "tool.yaml", "item_name: Tool\ncatalog: extras\ndisplay_name: Tool\nversion: 4.0\n")

	result, err := BuildCatalogs(repo)
	if err != nil {
		t.Fatalf("BuildCatalogs() error = %v", err)
	}
	if result.Records != 5 || result.Catalogs != 2 {
		t.Fatalf("BuildCatalogs() result = %#v, want 5 records and 2 catalogs", result)
	}

	base := readCatalog(t, repo, "base")
	if got := base["Chrome"].Version; got != "3.0" {
		t.Fatalf("Chrome version = %q, want 3.0", got)
	}
	if got := base["Chrome"].DisplayName; got != "New Chrome" {
		t.Fatalf("Chrome display_name = %q, want selected newest record", got)
	}
	if got := base["Agent"].Version; got != "7.0" {
		t.Fatalf("Agent version = %q, want 7.0", got)
	}
	if got := readCatalog(t, repo, "extras")["Tool"].Version; got != "4.0" {
		t.Fatalf("Tool version = %q, want 4.0", got)
	}
}

func TestBuildCatalogsPreservesCatalogItemFields(t *testing.T) {
	repo := t.TempDir()
	writePackageInfo(t, repo, "app.yaml", `
item_name: App
catalog: base
display_name: Example App
description: Example description
icon: icons/app.png
dependencies:
  - Dependency
check:
  script: checks/app.ps1
installer:
  type: exe
  location: packages/app/setup.exe
  hash: abc123
  arguments:
    - /quiet
uninstaller:
  type: exe
  location: packages/app/uninstall.exe
  arguments:
    - /quiet
blocking_apps:
  - app.exe
preinstall_script: scripts/pre.ps1
postinstall_script: scripts/post.ps1
version: 10.2.0
`)

	if _, err := BuildCatalogs(repo); err != nil {
		t.Fatalf("BuildCatalogs() error = %v", err)
	}
	item := readCatalog(t, repo, "base")["App"]
	if item.Icon != "icons/app.png" || item.Installer.Location != "packages/app/setup.exe" {
		t.Fatalf("representative fields were not preserved: %#v", item)
	}
	if len(item.Dependencies) != 1 || item.Dependencies[0] != "Dependency" {
		t.Fatalf("dependencies = %#v, want Dependency", item.Dependencies)
	}
	if item.Check.Script != "checks/app.ps1" || item.PreScript != "scripts/pre.ps1" || item.PostScript != "scripts/post.ps1" {
		t.Fatalf("script fields were not preserved: %#v", item)
	}
}

func TestBuildCatalogsValidationFailurePreservesExistingCatalogs(t *testing.T) {
	repo := t.TempDir()
	catalogs := filepath.Join(repo, "catalogs")
	if err := os.MkdirAll(catalogs, 0755); err != nil {
		t.Fatal(err)
	}
	existingPath := filepath.Join(catalogs, "existing.yaml")
	existing := []byte("Existing:\n  version: 1.0\n")
	if err := os.WriteFile(existingPath, existing, 0644); err != nil {
		t.Fatal(err)
	}
	writePackageInfo(t, repo, "invalid.yaml", "item_name: Broken\ncatalog: base\nversion: latest\n")

	if _, err := BuildCatalogs(repo); err == nil {
		t.Fatalf("BuildCatalogs() expected validation error")
	}
	got, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("existing catalog was removed after validation failure: %v", err)
	}
	if string(got) != string(existing) {
		t.Fatalf("existing catalog changed after validation failure: %q", got)
	}
}
