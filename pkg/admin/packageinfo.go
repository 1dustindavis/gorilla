package admin

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"go.yaml.in/yaml/v4"
)

type packageInfoRecord struct {
	Path     string
	ItemName string
	Catalog  string
	Item     catalog.Item
}

type packageInfoYAML struct {
	ItemName string       `yaml:"item_name"`
	Catalog  string       `yaml:"catalog"`
	Item     catalog.Item `yaml:",inline"`
}

type packageInfoIdentity struct {
	Catalog  string
	ItemName string
	Version  string
}

func loadPackageInfo(repoPath string) ([]packageInfoRecord, error) {
	packagesInfoPath := filepath.Join(repoPath, "packages-info")
	if _, err := os.Stat(packagesInfoPath); err != nil {
		return nil, fmt.Errorf("packages-info path unavailable: %w", err)
	}

	var paths []string
	if err := filepath.WalkDir(packagesInfoPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".yaml", ".yml":
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("discover package-info files: %w", err)
	}
	sort.Strings(paths)

	records := make([]packageInfoRecord, 0, len(paths))
	seen := make(map[packageInfoIdentity]string, len(paths))
	for _, path := range paths {
		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read package-info %s: %w", displayPackageInfoPath(repoPath, path), err)
		}

		var parsed packageInfoYAML
		if err := yaml.Unmarshal(contents, &parsed); err != nil {
			return nil, fmt.Errorf("parse package-info %s: %w", displayPackageInfoPath(repoPath, path), err)
		}

		parsed.ItemName = strings.TrimSpace(parsed.ItemName)
		parsed.Catalog = strings.TrimSpace(parsed.Catalog)
		parsed.Item.Version = strings.TrimSpace(parsed.Item.Version)
		displayPath := displayPackageInfoPath(repoPath, path)
		if parsed.ItemName == "" {
			return nil, fmt.Errorf("%s: item_name is required", displayPath)
		}
		if parsed.Catalog == "" {
			return nil, fmt.Errorf("%s: catalog is required", displayPath)
		}
		if parsed.Item.Version == "" {
			return nil, fmt.Errorf("%s: version is required", displayPath)
		}
		if _, err := parseVersion(parsed.Item.Version); err != nil {
			return nil, fmt.Errorf("%s: unsupported version %q", displayPath, parsed.Item.Version)
		}

		identity := packageInfoIdentity{
			Catalog:  parsed.Catalog,
			ItemName: parsed.ItemName,
			Version:  parsed.Item.Version,
		}
		if previousPath, ok := seen[identity]; ok {
			return nil, fmt.Errorf(
				"duplicate package-info (%s, %s, %s): %s conflicts with %s",
				parsed.Catalog,
				parsed.ItemName,
				parsed.Item.Version,
				previousPath,
				displayPath,
			)
		}
		seen[identity] = displayPath

		records = append(records, packageInfoRecord{
			Path:     path,
			ItemName: parsed.ItemName,
			Catalog:  parsed.Catalog,
			Item:     parsed.Item,
		})
	}
	return records, nil
}

func displayPackageInfoPath(repoPath, path string) string {
	relative, err := filepath.Rel(repoPath, path)
	if err != nil {
		return path
	}
	return filepath.Clean(relative)
}
