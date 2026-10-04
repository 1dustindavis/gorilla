package admin

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const DefaultKeepVersions = 3

type CleanupOptions struct {
	Keep int
}

type VersionDisposition int

const (
	VersionCurrent VersionDisposition = iota
	VersionRetained
	VersionSuperseded
	VersionAbandoned
)

func (d VersionDisposition) String() string {
	switch d {
	case VersionCurrent:
		return "CURRENT"
	case VersionRetained:
		return "RETAINED"
	case VersionSuperseded:
		return "SUPERSEDED"
	case VersionAbandoned:
		return "ABANDONED"
	default:
		return "UNKNOWN"
	}
}

type CleanupPlan struct {
	RepoPath       string
	Keep           int
	Items          []CleanupItem
	AbandonedFiles []string
	MissingAssets  []MissingAsset
}

type CleanupItem struct {
	Catalog  string
	ItemName string
	Live     bool
	Versions []CleanupVersion
}

type CleanupVersion struct {
	Version     string
	PackageInfo string
	Disposition VersionDisposition
}

type MissingAsset struct {
	Path         string
	ReferencedBy []CleanupAssetReference
}

type CleanupAssetReference struct {
	Catalog     string
	ItemName    string
	Version     string
	PackageInfo string
}

func PlanCleanup(repoPath string, options CleanupOptions) (CleanupPlan, error) {
	repoPath = filepath.Clean(repoPath)
	if options.Keep == 0 {
		options.Keep = DefaultKeepVersions
	}
	if options.Keep < 1 {
		return CleanupPlan{}, fmt.Errorf("keep must be at least 1")
	}

	records, err := loadPackageInfo(repoPath)
	if err != nil {
		return CleanupPlan{}, err
	}
	groups, err := groupPackageInfo(repoPath, records)
	if err != nil {
		return CleanupPlan{}, err
	}
	manifests, err := loadRepositoryManifests(repoPath)
	if err != nil {
		return CleanupPlan{}, err
	}

	live := determineLiveItems(groups, repositoryManifestRoots(manifests))
	plan := CleanupPlan{
		RepoPath: repoPath,
		Keep:     options.Keep,
		Items:    make([]CleanupItem, 0, len(groups)),
	}

	survivingReferences := make(map[string][]CleanupAssetReference)
	for _, key := range sortedPackageInfoKeys(groups) {
		group := groups[key]
		isLive := live[key]
		item := CleanupItem{
			Catalog:  key.Catalog,
			ItemName: key.ItemName,
			Live:     isLive,
			Versions: make([]CleanupVersion, 0, len(group)),
		}

		for i, record := range group {
			disposition := VersionAbandoned
			if isLive {
				switch {
				case i == 0:
					disposition = VersionCurrent
				case i < options.Keep:
					disposition = VersionRetained
				default:
					disposition = VersionSuperseded
				}
			}

			packageInfoPath := displayRepositoryPath(repoPath, record.Path)
			item.Versions = append(item.Versions, CleanupVersion{
				Version:     record.Item.Version,
				PackageInfo: packageInfoPath,
				Disposition: disposition,
			})

			refs, err := packageInfoAssetReferences(record)
			if err != nil {
				return CleanupPlan{}, fmt.Errorf(
					"validate asset references for %s/%s %s (%s): %w",
					record.Catalog,
					record.ItemName,
					record.Item.Version,
					packageInfoPath,
					err,
				)
			}
			if disposition == VersionCurrent || disposition == VersionRetained {
				for _, ref := range refs {
					survivingReferences[ref] = append(survivingReferences[ref], CleanupAssetReference{
						Catalog:     record.Catalog,
						ItemName:    record.ItemName,
						Version:     record.Item.Version,
						PackageInfo: packageInfoPath,
					})
				}
			}
		}
		plan.Items = append(plan.Items, item)
	}

	managedAssets, err := discoverManagedAssets(repoPath)
	if err != nil {
		return CleanupPlan{}, err
	}
	for _, asset := range managedAssets {
		if _, ok := survivingReferences[asset]; !ok {
			plan.AbandonedFiles = append(plan.AbandonedFiles, asset)
		}
	}

	missing, err := missingReferencedAssets(repoPath, survivingReferences)
	if err != nil {
		return CleanupPlan{}, err
	}
	plan.MissingAssets = missing
	return plan, nil
}

func determineLiveItems(groups map[packageInfoKey][]packageInfoRecord, roots []string) map[packageInfoKey]bool {
	byItemName := make(map[string][]packageInfoKey)
	for _, key := range sortedPackageInfoKeys(groups) {
		byItemName[key.ItemName] = append(byItemName[key.ItemName], key)
	}

	queue := append([]string(nil), roots...)
	seenNames := make(map[string]bool)
	live := make(map[packageInfoKey]bool)
	for len(queue) > 0 {
		itemName := queue[0]
		queue = queue[1:]
		if seenNames[itemName] {
			continue
		}
		seenNames[itemName] = true

		for _, key := range byItemName[itemName] {
			live[key] = true
			group := groups[key]
			if len(group) == 0 {
				continue
			}
			dependencies := append([]string(nil), group[0].Item.Dependencies...)
			for i := range dependencies {
				dependencies[i] = strings.TrimSpace(dependencies[i])
			}
			sort.Strings(dependencies)
			for _, dependency := range dependencies {
				if dependency != "" && !seenNames[dependency] {
					queue = append(queue, dependency)
				}
			}
		}
	}
	return live
}

func packageInfoAssetReferences(record packageInfoRecord) ([]string, error) {
	raw := []string{
		record.Item.Icon,
		record.Item.Check.Script,
		record.Item.Installer.Location,
		record.Item.Uninstaller.Location,
		record.Item.PreScript,
		record.Item.PostScript,
	}
	seen := make(map[string]struct{})
	for _, value := range raw {
		if strings.TrimSpace(value) == "" {
			continue
		}
		normalized, err := normalizeRepositoryAssetPath(value)
		if err != nil {
			return nil, err
		}
		seen[normalized] = struct{}{}
	}
	refs := make([]string, 0, len(seen))
	for ref := range seen {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	return refs, nil
}

func normalizeRepositoryAssetPath(value string) (string, error) {
	original := value
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("empty repository asset path")
	}
	if strings.ContainsRune(value, '\x00') {
		return "", fmt.Errorf("unsafe repository asset path %q: contains NUL", original)
	}

	portable := strings.ReplaceAll(value, `\`, "/")
	if strings.HasPrefix(portable, "/") || strings.Contains(portable, ":") {
		return "", fmt.Errorf("unsafe repository asset path %q: must be repository-relative", original)
	}
	cleaned := path.Clean(portable)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || path.IsAbs(cleaned) {
		return "", fmt.Errorf("unsafe repository asset path %q: resolves outside the repository", original)
	}
	return cleaned, nil
}

func discoverManagedAssets(repoPath string) ([]string, error) {
	var assets []string
	for _, rootName := range []string{"icons", "packages"} {
		root := filepath.Join(repoPath, rootName)
		info, err := os.Stat(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("inspect managed asset root %s: %w", rootName, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("managed asset root %s is not a directory", rootName)
		}

		if err := filepath.WalkDir(root, func(assetPath string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || d.Name() == ".gitkeep" {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			assets = append(assets, displayRepositoryPath(repoPath, assetPath))
			return nil
		}); err != nil {
			return nil, fmt.Errorf("discover managed assets under %s: %w", rootName, err)
		}
	}
	sort.Strings(assets)
	return assets, nil
}

func missingReferencedAssets(repoPath string, references map[string][]CleanupAssetReference) ([]MissingAsset, error) {
	paths := make([]string, 0, len(references))
	for assetPath := range references {
		paths = append(paths, assetPath)
	}
	sort.Strings(paths)

	missing := make([]MissingAsset, 0)
	for _, assetPath := range paths {
		fullPath := filepath.Join(repoPath, filepath.FromSlash(assetPath))
		_, err := os.Stat(fullPath)
		if err == nil {
			continue
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("inspect referenced asset %s: %w", assetPath, err)
		}

		refs := append([]CleanupAssetReference(nil), references[assetPath]...)
		sort.Slice(refs, func(i, j int) bool {
			if refs[i].Catalog != refs[j].Catalog {
				return refs[i].Catalog < refs[j].Catalog
			}
			if refs[i].ItemName != refs[j].ItemName {
				return refs[i].ItemName < refs[j].ItemName
			}
			if refs[i].Version != refs[j].Version {
				return refs[i].Version < refs[j].Version
			}
			return refs[i].PackageInfo < refs[j].PackageInfo
		})
		missing = append(missing, MissingAsset{Path: assetPath, ReferencedBy: refs})
	}
	return missing, nil
}
