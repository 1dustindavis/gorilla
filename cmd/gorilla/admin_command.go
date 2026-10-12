package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/1dustindavis/gorilla/pkg/admin"
)

var (
	adminBuildCatalogsFunc = admin.BuildCatalogs
	adminPlanCleanupFunc   = admin.PlanCleanup
	adminApplyCleanupFunc  = admin.ApplyCleanup
	adminGetwdFunc         = os.Getwd
)

func isAdminCommand(args []string) bool {
	return len(args) > 1 && args[1] == "admin"
}

func runAdmin(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gorilla admin <build|cleanup>")
	}

	switch args[0] {
	case "build":
		return runAdminBuild(args[1:], stdout)
	case "cleanup":
		return runAdminCleanup(args[1:], stdout)
	default:
		return fmt.Errorf("unknown admin command %q; usage: gorilla admin <build|cleanup>", args[0])
	}
}

func runAdminBuild(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("gorilla admin build", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	repo := flags.String("repo", "", "repository path")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("parse admin build arguments: %w", err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected admin build argument %q", flags.Arg(0))
	}

	repoPath, err := resolveAdminRepoPath(*repo)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Building catalogs from %s\n", repoPath)
	result, err := adminBuildCatalogsFunc(repoPath)
	if err != nil {
		return fmt.Errorf("build catalogs: %w", err)
	}
	fmt.Fprintf(stdout, "Loaded %d package-info records\n", result.Records)
	fmt.Fprintf(stdout, "Generated %d catalogs\n", result.Catalogs)
	return nil
}

func runAdminCleanup(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("gorilla admin cleanup", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	repo := flags.String("repo", "", "repository path")
	keep := flags.Int("keep", admin.DefaultKeepVersions, "package-info versions to retain per live item")
	apply := flags.Bool("apply", false, "apply the planned cleanup")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("parse admin cleanup arguments: %w", err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected admin cleanup argument %q", flags.Arg(0))
	}
	if *keep < 1 {
		return fmt.Errorf("--keep must be at least 1")
	}

	repoPath, err := resolveAdminRepoPath(*repo)
	if err != nil {
		return err
	}
	plan, err := adminPlanCleanupFunc(repoPath, admin.CleanupOptions{Keep: *keep})
	if err != nil {
		return fmt.Errorf("plan repository cleanup: %w", err)
	}
	writeCleanupPlan(stdout, plan)
	if !*apply {
		fmt.Fprintln(stdout, "\nDry run only. No files were changed.")
		return nil
	}

	fmt.Fprintln(stdout, "\nApplying cleanup...")
	result, err := adminApplyCleanupFunc(plan)
	if err != nil {
		return fmt.Errorf("apply repository cleanup: %w", err)
	}
	if result.PackageInfoRemoved == 0 && result.AssetsRemoved == 0 {
		fmt.Fprintln(stdout, "Nothing to remove.")
		fmt.Fprintln(stdout, "Repository was not changed.")
		return nil
	}

	fmt.Fprintln(stdout, "Removed:")
	fmt.Fprintf(stdout, "  %d package-info files\n", result.PackageInfoRemoved)
	fmt.Fprintf(stdout, "  %d abandoned asset files\n", result.AssetsRemoved)
	fmt.Fprintf(stdout, "  %d empty directories\n", result.DirectoriesRemoved)
	fmt.Fprintf(stdout, "Generated %d catalogs\n", result.CatalogsGenerated)
	fmt.Fprintln(stdout, "Repository cleanup complete.")
	return nil
}

func resolveAdminRepoPath(repo string) (string, error) {
	repoPath := strings.TrimSpace(repo)
	if repoPath == "" {
		cwd, err := adminGetwdFunc()
		if err != nil {
			return "", fmt.Errorf("determine current working directory: %w", err)
		}
		repoPath = cwd
	}
	return filepath.Clean(repoPath), nil
}

func writeCleanupPlan(stdout io.Writer, plan admin.CleanupPlan) {
	fmt.Fprintln(stdout, "Repository cleanup")
	fmt.Fprintf(stdout, "Repository: %s\n", plan.RepoPath)
	fmt.Fprintf(stdout, "Retention: %d versions per live item\n", plan.Keep)

	fmt.Fprintln(stdout, "\nActive items")
	for _, item := range plan.Items {
		if !item.Live {
			continue
		}
		fmt.Fprintf(stdout, "%s [%s]\n", item.ItemName, item.Catalog)
		for _, version := range item.Versions {
			fmt.Fprintf(stdout, "  %-12s %s\n", version.Disposition.String(), version.Version)
		}
	}

	fmt.Fprintln(stdout, "\nAbandoned items")
	for _, item := range plan.Items {
		if item.Live {
			continue
		}
		fmt.Fprintf(stdout, "%s [%s]\n", item.ItemName, item.Catalog)
		for _, version := range item.Versions {
			fmt.Fprintf(stdout, "  %-12s %s\n", version.Disposition.String(), version.Version)
		}
	}

	fmt.Fprintln(stdout, "\nAbandoned files")
	for _, assetPath := range plan.AbandonedFiles {
		fmt.Fprintf(stdout, "  %s\n", assetPath)
	}

	fmt.Fprintln(stdout, "\nMissing referenced files")
	for _, missing := range plan.MissingAssets {
		fmt.Fprintf(stdout, "  %s\n", missing.Path)
		for _, ref := range missing.ReferencedBy {
			fmt.Fprintf(stdout, "    referenced by %s / %s / %s\n", ref.ItemName, ref.Catalog, ref.Version)
		}
	}

	liveItems := 0
	abandonedItems := 0
	supersededPackageInfo := 0
	abandonedPackageInfo := 0
	for _, item := range plan.Items {
		if item.Live {
			liveItems++
		} else {
			abandonedItems++
		}
		for _, version := range item.Versions {
			switch version.Disposition {
			case admin.VersionSuperseded:
				supersededPackageInfo++
			case admin.VersionAbandoned:
				abandonedPackageInfo++
			}
		}
	}

	fmt.Fprintln(stdout, "\nSummary")
	fmt.Fprintf(stdout, "  %d live items\n", liveItems)
	fmt.Fprintf(stdout, "  %d abandoned items\n", abandonedItems)
	fmt.Fprintf(stdout, "  %d superseded package-info\n", supersededPackageInfo)
	fmt.Fprintf(stdout, "  %d abandoned package-info\n", abandonedPackageInfo)
	fmt.Fprintf(stdout, "  %d abandoned files\n", len(plan.AbandonedFiles))
	fmt.Fprintf(stdout, "  %d missing referenced files\n", len(plan.MissingAssets))
}
