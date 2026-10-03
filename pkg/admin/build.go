package admin

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"go.yaml.in/yaml/v4"
)

// BuildResult summarizes one repository catalog build.
type BuildResult struct {
	Records  int
	Catalogs int
}

type catalogItemKey struct {
	Catalog  string
	ItemName string
}

// BuildCatalogs validates all package-info records, selects the newest version
// of each catalog item, then replaces generated catalogs.
func BuildCatalogs(repoPath string) (BuildResult, error) {
	records, err := loadPackageInfo(repoPath)
	if err != nil {
		return BuildResult{}, err
	}

	selected := make(map[catalogItemKey]packageInfoRecord)
	for _, record := range records {
		key := catalogItemKey{Catalog: record.Catalog, ItemName: record.ItemName}
		current, ok := selected[key]
		if !ok {
			selected[key] = record
			continue
		}

		cmp, err := compareVersions(record.Item.Version, current.Item.Version)
		if err != nil {
			return BuildResult{}, fmt.Errorf("compare versions for %s/%s: %w", record.Catalog, record.ItemName, err)
		}
		if cmp > 0 || (cmp == 0 && record.Path > current.Path) {
			selected[key] = record
		}
	}

	byCatalog := make(map[string]map[string]catalog.Item)
	for key, record := range selected {
		if byCatalog[key.Catalog] == nil {
			byCatalog[key.Catalog] = make(map[string]catalog.Item)
		}
		byCatalog[key.Catalog][key.ItemName] = record.Item
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

	catalogsPath := filepath.Join(repoPath, "catalogs")
	if err := os.RemoveAll(catalogsPath); err != nil {
		return BuildResult{}, fmt.Errorf("clean catalogs path %s: %w", catalogsPath, err)
	}
	if err := os.MkdirAll(catalogsPath, 0755); err != nil {
		return BuildResult{}, fmt.Errorf("create catalogs path %s: %w", catalogsPath, err)
	}
	for _, catalogName := range catalogNames {
		catalogPath := filepath.Join(catalogsPath, catalogName+".yaml")
		if err := os.WriteFile(catalogPath, outputs[catalogName], 0644); err != nil {
			return BuildResult{}, fmt.Errorf("write catalog %s: %w", catalogPath, err)
		}
	}

	return BuildResult{Records: len(records), Catalogs: len(catalogNames)}, nil
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
