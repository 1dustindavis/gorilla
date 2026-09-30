package main

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/managed"
	"github.com/1dustindavis/gorilla/pkg/manifest"
	"github.com/1dustindavis/gorilla/pkg/process"
)

func TestManagedRunImportSuccessReturnsZeroPreparedState(t *testing.T) {
	resetMainHooks()
	defer resetMainHooks()

	cfg := config.Configuration{
		ImportArg:   "example.msi",
		CheckOnly:   true,
		RepoPath:    "repo/path",
		CachePath:   t.TempDir(),
		AppDataPath: t.TempDir(),
	}
	mkdirAllFunc = func(string, os.FileMode) error { return nil }
	importItemFunc = func(repoPath, itemPath string) error {
		if repoPath != cfg.RepoPath || itemPath != cfg.ImportArg {
			t.Fatalf("import args = %q, %q", repoPath, itemPath)
		}
		return nil
	}

	result, err := managedRun(cfg)
	if err != nil {
		t.Fatalf("managedRun(import) returned error: %v", err)
	}
	if !reflect.DeepEqual(result, managed.RunResult{}) {
		t.Fatalf("import mode result = %#v, want zero value", result)
	}
}

func TestManagedRunPreparationFailuresReturnZeroResult(t *testing.T) {
	tests := []struct {
		name string
		fail func()
		want string
	}{
		{
			name: "manifest",
			fail: func() {
				manifestGetFunc = func(config.Configuration) ([]manifest.Item, []string, error) {
					return nil, nil, errors.New("manifest boom")
				}
			},
			want: "unable to retrieve manifest: manifest boom",
		},
		{
			name: "catalog",
			fail: func() {
				catalogGetFunc = func(config.Configuration) (map[int]map[string]catalog.Item, error) {
					return nil, errors.New("catalog boom")
				}
			},
			want: "unable to retrieve catalog: catalog boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withManagedExecutionHooks(t)
			cfg := targetedTestConfig(t)
			tt.fail()

			result, err := managedRun(cfg)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
			if !reflect.DeepEqual(result, managed.RunResult{}) {
				t.Fatalf("failure result = %#v, want zero value", result)
			}
		})
	}
}

func TestManagedItemRunMissingRequestedResultReturnsZeroResult(t *testing.T) {
	withManagedExecutionHooks(t)
	cfg := targetedTestConfig(t)
	processInstallResultsFunc = func([]string, map[int]map[string]catalog.Item, string, string, bool) []process.ItemResult {
		return nil
	}

	result, err := managedItemRun(cfg, "AppB", "InstallItem")
	if err == nil || !strings.Contains(err.Error(), `targeted InstallItem returned no result for "AppB"`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(result, managed.ItemRunResult{}) {
		t.Fatalf("failure result = %#v, want zero value", result)
	}
}
