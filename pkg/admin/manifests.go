package admin

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v4"
)

type repositoryManifest struct {
	Path             string   `yaml:"-"`
	Name             string   `yaml:"name"`
	Includes         []string `yaml:"included_manifests"`
	Installs         []string `yaml:"managed_installs"`
	OptionalInstalls []string `yaml:"optional_installs"`
	Uninstalls       []string `yaml:"managed_uninstalls"`
	Updates          []string `yaml:"managed_updates"`
	Catalogs         []string `yaml:"catalogs"`
}

func loadRepositoryManifests(repoPath string) ([]repositoryManifest, error) {
	manifestsPath := filepath.Join(repoPath, "manifests")
	info, err := os.Stat(manifestsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("manifests directory does not exist: %s", manifestsPath)
		}
		return nil, fmt.Errorf("inspect manifests directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("manifests path is not a directory: %s", manifestsPath)
	}

	var paths []string
	if err := filepath.WalkDir(manifestsPath, func(path string, d os.DirEntry, walkErr error) error {
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
		return nil, fmt.Errorf("discover repository manifests: %w", err)
	}
	sort.Strings(paths)

	manifests := make([]repositoryManifest, 0, len(paths))
	for _, path := range paths {
		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read repository manifest %s: %w", displayRepositoryPath(repoPath, path), err)
		}
		var parsed repositoryManifest
		if err := yaml.Unmarshal(contents, &parsed); err != nil {
			return nil, fmt.Errorf("parse repository manifest %s: %w", displayRepositoryPath(repoPath, path), err)
		}
		parsed.Path = displayRepositoryPath(repoPath, path)
		manifests = append(manifests, parsed)
	}
	return manifests, nil
}

func repositoryManifestRoots(manifests []repositoryManifest) []string {
	seen := make(map[string]struct{})
	for _, manifest := range manifests {
		lists := [][]string{
			manifest.Installs,
			manifest.OptionalInstalls,
			manifest.Uninstalls,
			manifest.Updates,
		}
		for _, items := range lists {
			for _, itemName := range items {
				itemName = strings.TrimSpace(itemName)
				if itemName != "" {
					seen[itemName] = struct{}{}
				}
			}
		}
	}

	roots := make([]string, 0, len(seen))
	for itemName := range seen {
		roots = append(roots, itemName)
	}
	sort.Strings(roots)
	return roots
}

func displayRepositoryPath(repoPath, path string) string {
	relative, err := filepath.Rel(repoPath, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(filepath.Clean(relative))
}
