package admin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePackageInfo(t *testing.T, repo, relative, contents string) string {
	t.Helper()
	path := filepath.Join(repo, "packages-info", relative)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadPackageInfoAcceptsYAMLAndYML(t *testing.T) {
	repo := t.TempDir()
	writePackageInfo(t, repo, "z.yaml", "item_name: Z\ncatalog: base\nversion: 2.0\n")
	writePackageInfo(t, repo, filepath.Join("nested", "a.yml"), "item_name: A\ncatalog: extras\nversion: 1.0\n")
	writePackageInfo(t, repo, "ignored.txt", "not yaml")

	records, err := loadPackageInfo(repo)
	if err != nil {
		t.Fatalf("loadPackageInfo() error = %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("len(records) = %d, want 2", len(records))
	}
	if records[0].ItemName != "A" || records[0].Catalog != "extras" || records[0].Item.Version != "1.0" {
		t.Fatalf("unexpected first record: %#v", records[0])
	}
	if records[1].ItemName != "Z" || records[1].Catalog != "base" || records[1].Item.Version != "2.0" {
		t.Fatalf("unexpected second record: %#v", records[1])
	}
}

func TestLoadPackageInfoValidation(t *testing.T) {
	tests := []struct {
		name     string
		contents string
		want     string
	}{
		{name: "missing item name", contents: "catalog: base\nversion: 1.0\n", want: "item_name is required"},
		{name: "missing catalog", contents: "item_name: Chrome\nversion: 1.0\n", want: "catalog is required"},
		{name: "missing version", contents: "item_name: Chrome\ncatalog: base\n", want: "version is required"},
		{name: "unsupported version", contents: "item_name: Chrome\ncatalog: base\nversion: latest\n", want: "unsupported version \"latest\""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := t.TempDir()
			writePackageInfo(t, repo, "invalid.yaml", tt.contents)
			_, err := loadPackageInfo(repo)
			if err == nil {
				t.Fatalf("loadPackageInfo() expected error")
			}
			if !strings.Contains(err.Error(), filepath.Clean(filepath.Join("packages-info", "invalid.yaml"))) {
				t.Fatalf("error %q does not identify source file", err)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want substring %q", err, tt.want)
			}
		})
	}
}

func TestLoadPackageInfoRejectsDuplicateIdentityVersion(t *testing.T) {
	repo := t.TempDir()
	writePackageInfo(t, repo, "chrome-a.yaml", "item_name: Chrome\ncatalog: base\nversion: 145.0\n")
	writePackageInfo(t, repo, "chrome-b.yaml", "item_name: Chrome\ncatalog: base\nversion: 145.0\n")

	_, err := loadPackageInfo(repo)
	if err == nil {
		t.Fatalf("loadPackageInfo() expected duplicate error")
	}
	for _, want := range []string{"duplicate package-info", "chrome-a.yaml", "chrome-b.yaml"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want substring %q", err, want)
		}
	}
}
