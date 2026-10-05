package admin

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

var (
	cleanupBuildCatalogs = BuildCatalogs
	cleanupLstat         = os.Lstat
	cleanupMkdirAll      = os.MkdirAll
	cleanupMkdirTemp     = os.MkdirTemp
	cleanupReadDir       = os.ReadDir
	cleanupRemove        = os.Remove
	cleanupRemoveAll     = os.RemoveAll
	cleanupRename        = os.Rename
)

// CleanupResult summarizes one applied repository cleanup.
type CleanupResult struct {
	PackageInfoRemoved int
	AssetsRemoved      int
	DirectoriesRemoved int
	CatalogsGenerated  int
}

type cleanupCandidateKind int

const (
	cleanupPackageInfo cleanupCandidateKind = iota
	cleanupAsset
)

type cleanupCandidate struct {
	Relative string
	FullPath string
	Kind     cleanupCandidateKind
}

type stagedCleanupCandidate struct {
	cleanupCandidate
	StagedPath string
}

// ApplyCleanup validates and applies an already-created cleanup plan. It does
// not perform reachability or retention classification; PlanCleanup remains the
// source of truth for those decisions.
func ApplyCleanup(plan CleanupPlan) (CleanupResult, error) {
	if strings.TrimSpace(plan.RepoPath) == "" {
		return CleanupResult{}, fmt.Errorf("cleanup plan repository path is required")
	}
	plan.RepoPath = filepath.Clean(strings.TrimSpace(plan.RepoPath))
	candidates, err := validateCleanupPlan(plan)
	if err != nil {
		return CleanupResult{}, err
	}
	if len(candidates) == 0 {
		return CleanupResult{}, nil
	}

	stagingPath, err := cleanupMkdirTemp(plan.RepoPath, ".gorilla-cleanup-*")
	if err != nil {
		return CleanupResult{}, fmt.Errorf("create cleanup staging directory: %w", err)
	}

	moved := make([]stagedCleanupCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		stagedPath := filepath.Join(stagingPath, filepath.FromSlash(candidate.Relative))
		if err := cleanupMkdirAll(filepath.Dir(stagedPath), 0755); err != nil {
			return CleanupResult{}, rollbackCleanup(
				moved,
				stagingPath,
				fmt.Errorf("create staging parent for %s: %w", candidate.Relative, err),
			)
		}
		if err := cleanupRename(candidate.FullPath, stagedPath); err != nil {
			return CleanupResult{}, rollbackCleanup(
				moved,
				stagingPath,
				fmt.Errorf("stage cleanup candidate %s: %w", candidate.Relative, err),
			)
		}
		moved = append(moved, stagedCleanupCandidate{cleanupCandidate: candidate, StagedPath: stagedPath})
	}

	buildResult, buildErr := cleanupBuildCatalogs(plan.RepoPath)
	if buildErr != nil && !catalogBuildWasCommitted(buildErr) {
		return CleanupResult{}, rollbackCleanup(moved, stagingPath, fmt.Errorf("rebuild catalogs: %w", buildErr))
	}

	result := cleanupResultForCandidates(candidates)
	result.CatalogsGenerated = buildResult.Catalogs

	if err := cleanupRemoveAll(stagingPath); err != nil {
		if buildErr != nil {
			return result, fmt.Errorf(
				"cleanup and catalog replacement were applied, but catalog finalization failed (%v) and staged content could not be removed at %s: %w",
				buildErr,
				stagingPath,
				err,
			)
		}
		return result, fmt.Errorf("cleanup was applied, but staged content could not be removed at %s: %w", stagingPath, err)
	}

	directoriesRemoved, dirErr := removeEmptyManagedDirectories(plan.RepoPath)
	result.DirectoriesRemoved = directoriesRemoved
	if buildErr != nil {
		if dirErr != nil {
			return result, fmt.Errorf("cleanup was applied, but catalog finalization failed: %v; remove empty managed directories: %w", buildErr, dirErr)
		}
		return result, fmt.Errorf("cleanup was applied, but catalog finalization failed: %w", buildErr)
	}
	if dirErr != nil {
		return result, fmt.Errorf("cleanup was applied, but remove empty managed directories failed: %w", dirErr)
	}

	return result, nil
}

func cleanupResultForCandidates(candidates []cleanupCandidate) CleanupResult {
	var result CleanupResult
	for _, candidate := range candidates {
		switch candidate.Kind {
		case cleanupPackageInfo:
			result.PackageInfoRemoved++
		case cleanupAsset:
			result.AssetsRemoved++
		}
	}
	return result
}

func validateCleanupPlan(plan CleanupPlan) ([]cleanupCandidate, error) {
	repoPath := plan.RepoPath

	repoInfo, err := cleanupLstat(repoPath)
	if err != nil {
		return nil, fmt.Errorf("inspect repository root %s: %w", repoPath, err)
	}
	if repoInfo.Mode()&os.ModeSymlink != 0 || !repoInfo.IsDir() {
		return nil, fmt.Errorf("repository root %s must be a real directory", repoPath)
	}

	var raw []struct {
		path string
		kind cleanupCandidateKind
	}
	for _, item := range plan.Items {
		for _, version := range item.Versions {
			switch version.Disposition {
			case VersionSuperseded, VersionAbandoned:
				raw = append(raw, struct {
					path string
					kind cleanupCandidateKind
				}{path: version.PackageInfo, kind: cleanupPackageInfo})
			}
		}
	}
	for _, assetPath := range plan.AbandonedFiles {
		raw = append(raw, struct {
			path string
			kind cleanupCandidateKind
		}{path: assetPath, kind: cleanupAsset})
	}

	seen := make(map[string]cleanupCandidateKind, len(raw))
	candidates := make([]cleanupCandidate, 0, len(raw))
	for _, candidate := range raw {
		normalized, err := normalizeCleanupCandidate(candidate.path, candidate.kind)
		if err != nil {
			return nil, err
		}
		if previousKind, ok := seen[normalized]; ok {
			if previousKind == candidate.kind {
				return nil, fmt.Errorf("duplicate cleanup candidate %s", normalized)
			}
			return nil, fmt.Errorf("conflicting cleanup candidate %s", normalized)
		}
		seen[normalized] = candidate.kind

		fullPath := filepath.Join(repoPath, filepath.FromSlash(normalized))
		relative, err := filepath.Rel(repoPath, fullPath)
		if err != nil {
			return nil, fmt.Errorf("resolve cleanup candidate %s: %w", normalized, err)
		}
		if filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			return nil, fmt.Errorf("cleanup candidate %s resolves outside repository root", normalized)
		}
		if err := validateCleanupCandidateFilesystem(repoPath, normalized, fullPath); err != nil {
			return nil, err
		}

		candidates = append(candidates, cleanupCandidate{Relative: normalized, FullPath: fullPath, Kind: candidate.kind})
	}

	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Relative < candidates[j].Relative })
	return candidates, nil
}

func normalizeCleanupCandidate(value string, kind cleanupCandidateKind) (string, error) {
	original := value
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsRune(value, '\x00') {
		return "", fmt.Errorf("unsafe cleanup candidate %q", original)
	}
	portable := strings.ReplaceAll(value, `\`, "/")
	if strings.HasPrefix(portable, "/") || strings.Contains(portable, ":") || filepath.IsAbs(value) || filepath.VolumeName(value) != "" {
		return "", fmt.Errorf("unsafe cleanup candidate %q: must be repository-relative", original)
	}
	for _, component := range strings.Split(portable, "/") {
		if component == ".." {
			return "", fmt.Errorf("unsafe cleanup candidate %q: path traversal is not allowed", original)
		}
	}
	cleaned := path.Clean(portable)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || path.IsAbs(cleaned) {
		return "", fmt.Errorf("unsafe cleanup candidate %q: path traversal is not allowed", original)
	}

	root := strings.SplitN(cleaned, "/", 2)[0]
	switch kind {
	case cleanupPackageInfo:
		if root != "packages-info" || cleaned == "packages-info" {
			return "", fmt.Errorf("unsafe package-info cleanup candidate %q: must be beneath packages-info/", original)
		}
	case cleanupAsset:
		if (root != "packages" && root != "icons") || cleaned == root {
			return "", fmt.Errorf("unsafe asset cleanup candidate %q: must be beneath packages/ or icons/", original)
		}
	default:
		return "", fmt.Errorf("unknown cleanup candidate kind for %q", original)
	}
	return cleaned, nil
}

func validateCleanupCandidateFilesystem(repoPath, relative, fullPath string) error {
	parts := strings.Split(relative, "/")
	current := repoPath
	for i, part := range parts {
		current = filepath.Join(current, filepath.FromSlash(part))
		info, err := cleanupLstat(current)
		if err != nil {
			return fmt.Errorf("inspect cleanup candidate %s: %w", relative, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("cleanup candidate %s contains a symlink at %s", relative, displayRepositoryPath(repoPath, current))
		}
		if i < len(parts)-1 && !info.IsDir() {
			return fmt.Errorf("cleanup candidate %s has non-directory parent %s", relative, displayRepositoryPath(repoPath, current))
		}
	}
	info, err := cleanupLstat(fullPath)
	if err != nil {
		return fmt.Errorf("inspect cleanup candidate %s: %w", relative, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("cleanup candidate %s is not a regular file", relative)
	}
	return nil
}

func rollbackCleanup(moved []stagedCleanupCandidate, stagingPath string, originalErr error) error {
	var rollbackErrs []error
	for i := len(moved) - 1; i >= 0; i-- {
		candidate := moved[i]
		if err := cleanupMkdirAll(filepath.Dir(candidate.FullPath), 0755); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("recreate parent for %s: %w", candidate.Relative, err))
			continue
		}
		if err := cleanupRename(candidate.StagedPath, candidate.FullPath); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("restore %s: %w", candidate.Relative, err))
		}
	}
	if len(rollbackErrs) == 0 {
		if err := cleanupRemoveAll(stagingPath); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("remove staging directory %s: %w", stagingPath, err))
		}
	}
	if len(rollbackErrs) == 0 {
		return originalErr
	}
	return fmt.Errorf("%w; rollback failed for staging directory %s: %v", originalErr, stagingPath, errors.Join(rollbackErrs...))
}

func removeEmptyManagedDirectories(repoPath string) (int, error) {
	var directories []string
	for _, rootName := range []string{"packages", "icons", "packages-info"} {
		root := filepath.Join(repoPath, rootName)
		info, err := cleanupLstat(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return 0, fmt.Errorf("inspect managed root %s: %w", rootName, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return 0, fmt.Errorf("managed root %s is not a real directory", rootName)
		}

		err = filepath.WalkDir(root, func(dirPath string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if dirPath == root {
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.IsDir() {
				directories = append(directories, dirPath)
			}
			return nil
		})
		if err != nil {
			return 0, fmt.Errorf("discover managed directories under %s: %w", rootName, err)
		}
	}

	sort.Slice(directories, func(i, j int) bool {
		leftDepth := strings.Count(filepath.Clean(directories[i]), string(os.PathSeparator))
		rightDepth := strings.Count(filepath.Clean(directories[j]), string(os.PathSeparator))
		if leftDepth != rightDepth {
			return leftDepth > rightDepth
		}
		return directories[i] > directories[j]
	})

	removed := 0
	for _, dirPath := range directories {
		entries, err := cleanupReadDir(dirPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return removed, fmt.Errorf("inspect managed directory %s: %w", displayRepositoryPath(repoPath, dirPath), err)
		}
		if len(entries) != 0 {
			continue
		}
		if err := cleanupRemove(dirPath); err != nil {
			return removed, fmt.Errorf("remove empty managed directory %s: %w", displayRepositoryPath(repoPath, dirPath), err)
		}
		removed++
	}
	return removed, nil
}
