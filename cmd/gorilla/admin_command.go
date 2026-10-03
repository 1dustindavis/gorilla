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
	adminGetwdFunc         = os.Getwd
)

func isAdminCommand(args []string) bool {
	return len(args) > 1 && args[1] == "admin"
}

func runAdmin(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gorilla admin build [--repo <path>]")
	}

	switch args[0] {
	case "build":
		return runAdminBuild(args[1:], stdout)
	default:
		return fmt.Errorf("unknown admin command %q; usage: gorilla admin build [--repo <path>]", args[0])
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

	repoPath := strings.TrimSpace(*repo)
	if repoPath == "" {
		cwd, err := adminGetwdFunc()
		if err != nil {
			return fmt.Errorf("determine current working directory: %w", err)
		}
		repoPath = cwd
	}
	repoPath = filepath.Clean(repoPath)

	fmt.Fprintf(stdout, "Building catalogs from %s\n", repoPath)
	result, err := adminBuildCatalogsFunc(repoPath)
	if err != nil {
		return fmt.Errorf("build catalogs: %w", err)
	}
	fmt.Fprintf(stdout, "Loaded %d package-info records\n", result.Records)
	fmt.Fprintf(stdout, "Generated %d catalogs\n", result.Catalogs)
	return nil
}
