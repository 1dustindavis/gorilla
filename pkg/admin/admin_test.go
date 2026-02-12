package admin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"go.yaml.in/yaml/v4"
)

func TestBuildCatalogs(t *testing.T) {
	repoPath := t.TempDir()
	packagesInfoPath := filepath.Join(repoPath, "packages-info")
	if err := os.MkdirAll(packagesInfoPath, 0755); err != nil {
		t.Fatal(err)
	}

	itemA := `
item_name: Chrome
display_name: Google Chrome
catalog: base
installer:
  type: nupkg
  location: packages/chrome/chrome.nupkg
  hash: abc
`
	itemB := `
display_name: Agent Tool
catalog: base
installer:
  type: nupkg
  location: packages/agent/agent.nupkg
  hash: def
`
	itemSkip := `
display_name: No Catalog
installer:
  type: nupkg
  location: packages/skip/skip.nupkg
  hash: ghi
`
	if err := os.WriteFile(filepath.Join(packagesInfoPath, "chrome.yaml"), []byte(itemA), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packagesInfoPath, "agent.yaml"), []byte(itemB), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packagesInfoPath, "skip.yaml"), []byte(itemSkip), 0644); err != nil {
		t.Fatal(err)
	}

	if err := BuildCatalogs(repoPath); err != nil {
		t.Fatalf("BuildCatalogs failed: %v", err)
	}

	catalogYAML, err := os.ReadFile(filepath.Join(repoPath, "catalogs", "base.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]catalog.Item
	if err = yaml.Unmarshal(catalogYAML, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["Chrome"]; !ok {
		t.Fatalf("expected item key Chrome in generated catalog")
	}
	if _, ok := got["AgentTool"]; !ok {
		t.Fatalf("expected fallback item key AgentTool in generated catalog")
	}
	if _, ok := got["NoCatalog"]; ok {
		t.Fatalf("did not expect item without catalog to be generated")
	}
}

func TestBuildCatalogsMissingPackagesInfo(t *testing.T) {
	repoPath := t.TempDir()
	if err := BuildCatalogs(repoPath); err == nil {
		t.Fatalf("expected error when packages-info is missing")
	}
}
