package admin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"go.yaml.in/yaml/v4"
)

func TestBuildCatalogsPreservesOptionalIcon(t *testing.T) {
	repo := t.TempDir()
	packagesInfo := filepath.Join(repo, "packages-info")
	if err := os.MkdirAll(packagesInfo, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packagesInfo, "chrome.yaml"), []byte(`item_name: Chrome
catalog: base
display_name: Google Chrome
icon: icons/google-chrome.png
version: 1.2.3
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packagesInfo, "plain.yaml"), []byte(`item_name: Plain
catalog: base
display_name: Plain App
version: 1.0
`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := BuildCatalogs(repo); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(repo, "catalogs", "base.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]catalog.Item
	if err := yaml.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got["Chrome"].Icon != "icons/google-chrome.png" {
		t.Fatalf("icon = %q, want preserved package-info value", got["Chrome"].Icon)
	}
	if got["Plain"].Icon != "" {
		t.Fatalf("omitted icon = %q, want empty", got["Plain"].Icon)
	}
}
