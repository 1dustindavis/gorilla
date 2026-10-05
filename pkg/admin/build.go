package admin

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"go.yaml.in/yaml/v4"
)

var (
	adminMkdirTemp = os.MkdirTemp
	adminRemoveAll = os.RemoveAll
	adminRename    = os.Rename
	adminWriteFile = os.WriteFile
)

// BuildResult summarizes one repository catalog build.
type BuildResult struct {
	Records  int
	Catalogs int
}

type catalogBuildError struct {
	err       error
	committed bool
}

func (e *catalogBuildError) Error() string { return e.err.Error() }
func (e *catalogBuildError) Unwrap() error { return e.err }

func catalogBuildWasCommitted(err error) bool {
	var buildErr *catalogBuildError
	return errors.As(err, &buildErr) && buildErr.committed
}

// BuildCatalogs validates all package-info records, selects the newest version
// of each catalog item, then replaces generated catalogs.
func BuildCatalogs(repoPath string) (BuildResult, error) {
	records, err := loadPackageInfo(repoPath)
	if err != nil {
		return BuildResult{}, err
	}
	groups, err := groupPackageInfo(repoPath, records)
	if err != nil {
		return BuildResult{}, err
	}

	byCatalog := make(map[string]map[string]catalog.Item)
	for _, key := range sortedPackageInfoKeys(groups) {
		group := groups[key]
		if len(group) == 0 {
			continue
		}
		if byCatalog[key.Catalog] == nil {
			byCatalog[key.Catalog] = make(map[string]catalog.Item)
		}
		byCatalog[key.Catalog][key.ItemName] = group[0].Item
	}

	catalogNames := make([]string, 0, len(byCatalog))
	for name := range byCatalog {
		catalogNames = append(catalogNames, name)
	}
	sort.Strings(catalogNames)

	outputs := make(map[string][]byte, len(catalogNames))
	for _, catalogName := range catalogNames {
		contents, err := marshalCatalog(byCatalog[catalogName])
		if err != nil {
			return BuildResult{}, fmt.Errorf("marshal catalog %s: %w", catalogName, err)
		}
		outputs[catalogName] = contents
	}

	result := BuildResult{Records: len(records), Catalogs: len(catalogNames)}
	if err := replaceCatalogs(repoPath, catalogNames, outputs); err != nil {
		if catalogBuildWasCommitted(err) {
			return result, err
		}
		return BuildResult{}, err
	}

	return result, nil
}

func replaceCatalogs(repoPath string, catalogNames []string, outputs map[string][]byte) error {
	catalogsPath := filepath.Join(repoPath, "catalogs")
	tempPath, err := adminMkdirTemp(repoPath, ".catalogs-build-*")
	if err != nil {
		return fmt.Errorf("create temporary catalogs directory: %w", err)
	}
	defer adminRemoveAll(tempPath)

	for _, catalogName := range catalogNames {
		catalogPath, err := catalogOutputPath(tempPath, catalogName)
		if err != nil {
			return err
		}
		if err := adminWriteFile(catalogPath, outputs[catalogName], 0644); err != nil {
			return fmt.Errorf("write temporary catalog %s: %w", catalogPath, err)
		}
	}

	backupPath := tempPath + "-previous"
	hadExisting := false
	if _, err := os.Stat(catalogsPath); err == nil {
		hadExisting = true
		if err := adminRename(catalogsPath, backupPath); err != nil {
			return fmt.Errorf("preserve existing catalogs directory: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect existing catalogs directory: %w", err)
	}

	if err := adminRename(tempPath, catalogsPath); err != nil {
		if hadExisting {
			if restoreErr := adminRename(backupPath, catalogsPath); restoreErr != nil {
				return fmt.Errorf("activate generated catalogs: %w; restore previous catalogs: %v", err, restoreErr)
			}
		}
		return fmt.Errorf("activate generated catalogs: %w", err)
	}

	if hadExisting {
		if err := adminRemoveAll(backupPath); err != nil {
			return &catalogBuildError{
				err:       fmt.Errorf("remove previous catalogs backup: %w", err),
				committed: true,
			}
		}
	}
	return nil
}

func catalogOutputPath(catalogsPath, catalogName string) (string, error) {
	candidate := filepath.Join(catalogsPath, catalogName+".yaml")
	relative, err := filepath.Rel(catalogsPath, candidate)
	if err != nil {
		return "", fmt.Errorf("resolve catalog output path for %q: %w", catalogName, err)
	}
	if filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("catalog %q resolves outside catalogs directory", catalogName)
	}
	return candidate, nil
}

func marshalCatalog(items map[string]catalog.Item) ([]byte, error) {
	itemNames := make([]string, 0, len(items))
	for itemName := range items {
		itemNames = append(itemNames, itemName)
	}
	sort.Strings(itemNames)

	root := &yaml.Node{Kind: yaml.MappingNode}
	for _, itemName := range itemNames {
		key := &yaml.Node{Kind: yaml.ScalarNode, Value: itemName}
		value := &yaml.Node{}
		if err := value.Encode(items[itemName]); err != nil {
			return nil, err
		}
		root.Content = append(root.Content, key, value)
	}
	return yaml.Marshal(root)
}
