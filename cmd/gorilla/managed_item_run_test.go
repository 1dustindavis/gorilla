package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/installer"
	"github.com/1dustindavis/gorilla/pkg/manifest"
	"github.com/1dustindavis/gorilla/pkg/process"
)

func withManagedExecutionHooks(t *testing.T) {
	t.Helper()

	oldManifestGet := manifestGetFunc
	oldCatalogGet := catalogGetFunc
	oldProcessManifests := processManifestsFunc
	oldInstall := processInstallResultsFunc
	oldUninstall := processUninstallResultsFunc
	oldUpdate := processUpdateResultsFunc
	oldCleanup := processCleanUpFunc
	oldResetStatus := statusResetRegistryCacheFunc
	oldAdminCheck := adminCheckFunc

	t.Cleanup(func() {
		manifestGetFunc = oldManifestGet
		catalogGetFunc = oldCatalogGet
		processManifestsFunc = oldProcessManifests
		processInstallResultsFunc = oldInstall
		processUninstallResultsFunc = oldUninstall
		processUpdateResultsFunc = oldUpdate
		processCleanUpFunc = oldCleanup
		statusResetRegistryCacheFunc = oldResetStatus
		adminCheckFunc = oldAdminCheck
	})

	adminCheckFunc = func() (bool, error) { return true, nil }
	manifestGetFunc = func(config.Configuration) ([]manifest.Item, []string, error) {
		return []manifest.Item{{}}, []string{"manifest-catalog"}, nil
	}
	catalogGetFunc = func(cfg config.Configuration) (map[int]map[string]catalog.Item, error) {
		return map[int]map[string]catalog.Item{0: {}}, nil
	}
	processCleanUpFunc = func(string) {}
	statusResetRegistryCacheFunc = func() {}
}

func targetedTestConfig(t *testing.T) config.Configuration {
	t.Helper()
	return config.Configuration{
		CheckOnly:   true,
		CachePath:   t.TempDir(),
		AppDataPath: t.TempDir(),
	}
}

func TestManagedItemRunInstallUsesRequestedRootOnly(t *testing.T) {
	withManagedExecutionHooks(t)
	cfg := targetedTestConfig(t)

	var gotInstalls []string
	processInstallResultsFunc = func(installs []string, _ map[int]map[string]catalog.Item, _, _ string, _ bool) []process.ItemResult {
		gotInstalls = append([]string(nil), installs...)
		return []process.ItemResult{{
			ItemName: "AppB",
			Result: installer.Result{
				ItemName: "AppB",
				Action:   "install",
				Outcome:  installer.OutcomeSucceeded,
			},
		}}
	}
	processUninstallResultsFunc = func([]string, map[int]map[string]catalog.Item, string, string, bool) []process.ItemResult {
		t.Fatal("targeted install must not execute uninstalls")
		return nil
	}
	processUpdateResultsFunc = func([]string, map[int]map[string]catalog.Item, string, string, bool) []process.ItemResult {
		t.Fatal("targeted install must not execute updates")
		return nil
	}
	processManifestsFunc = func([]manifest.Item, map[int]map[string]catalog.Item) ([]string, []string, []string) {
		t.Fatal("targeted execution must not derive its work set from process.Manifests")
		return nil, nil, nil
	}

	result, err := managedItemRun(cfg, "AppB", "InstallItem")
	if err != nil {
		t.Fatalf("managedItemRun returned error: %v", err)
	}
	if want := []string{"AppB"}; !reflect.DeepEqual(gotInstalls, want) {
		t.Fatalf("install work set = %v, want %v", gotInstalls, want)
	}
	if result.Outcome != installer.OutcomeSucceeded {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestManagedItemRunInstallReturnsRootDependencyFailure(t *testing.T) {
	withManagedExecutionHooks(t)
	cfg := targetedTestConfig(t)

	processInstallResultsFunc = func(installs []string, _ map[int]map[string]catalog.Item, _, _ string, _ bool) []process.ItemResult {
		if want := []string{"AppB"}; !reflect.DeepEqual(installs, want) {
			t.Fatalf("install work set = %v, want %v", installs, want)
		}
		return []process.ItemResult{
			{
				ItemName: "RuntimeX",
				Result: installer.Result{ItemName: "RuntimeX", Action: "install", Outcome: installer.OutcomeFailed, ErrorCode: "invalid_dependency"},
			},
			{
				ItemName: "AppB",
				Result: installer.Result{ItemName: "AppB", Action: "install", Outcome: installer.OutcomeFailed, ErrorCode: "dependency_failed"},
			},
		}
	}

	result, err := managedItemRun(cfg, "AppB", "InstallItem")
	if err != nil {
		t.Fatalf("managedItemRun returned error: %v", err)
	}
	if result.ErrorCode != "dependency_failed" || result.Outcome != installer.OutcomeFailed {
		t.Fatalf("unexpected root dependency result: %#v", result)
	}
}

func TestManagedItemRunRemoveUsesRequestedItemOnly(t *testing.T) {
	withManagedExecutionHooks(t)
	cfg := targetedTestConfig(t)

	processInstallResultsFunc = func([]string, map[int]map[string]catalog.Item, string, string, bool) []process.ItemResult {
		t.Fatal("targeted remove must not execute installs")
		return nil
	}
	processUpdateResultsFunc = func([]string, map[int]map[string]catalog.Item, string, string, bool) []process.ItemResult {
		t.Fatal("targeted remove must not execute updates")
		return nil
	}
	var gotUninstalls []string
	processUninstallResultsFunc = func(uninstalls []string, _ map[int]map[string]catalog.Item, _, _ string, _ bool) []process.ItemResult {
		gotUninstalls = append([]string(nil), uninstalls...)
		return []process.ItemResult{{
			ItemName: "AppB",
			Result: installer.Result{ItemName: "AppB", Action: "uninstall", Outcome: installer.OutcomeSucceeded},
		}}
	}

	result, err := managedItemRun(cfg, "AppB", "RemoveItem")
	if err != nil {
		t.Fatalf("managedItemRun returned error: %v", err)
	}
	if want := []string{"AppB"}; !reflect.DeepEqual(gotUninstalls, want) {
		t.Fatalf("uninstall work set = %v, want %v", gotUninstalls, want)
	}
	if result.Action != "uninstall" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestManagedItemRunRejectsUnsupportedActionBeforeExecution(t *testing.T) {
	withManagedExecutionHooks(t)
	cfg := targetedTestConfig(t)

	manifestGetFunc = func(config.Configuration) ([]manifest.Item, []string, error) {
		t.Fatal("unsupported action must be rejected before managed context loading")
		return nil, nil, nil
	}

	_, err := managedItemRun(cfg, "AppB", "UpdateItem")
	if err == nil || !strings.Contains(err.Error(), "unsupported targeted managed item action") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestManagedRunRemainsFullConvergence(t *testing.T) {
	withManagedExecutionHooks(t)
	cfg := targetedTestConfig(t)

	processManifestsFunc = func([]manifest.Item, map[int]map[string]catalog.Item) ([]string, []string, []string) {
		return []string{"AppA", "AppB"}, []string{"AppC"}, []string{"AppD"}
	}

	var installs, uninstalls, updates []string
	processInstallResultsFunc = func(items []string, _ map[int]map[string]catalog.Item, _, _ string, _ bool) []process.ItemResult {
		installs = append([]string(nil), items...)
		return nil
	}
	processUninstallResultsFunc = func(items []string, _ map[int]map[string]catalog.Item, _, _ string, _ bool) []process.ItemResult {
		uninstalls = append([]string(nil), items...)
		return nil
	}
	processUpdateResultsFunc = func(items []string, _ map[int]map[string]catalog.Item, _, _ string, _ bool) []process.ItemResult {
		updates = append([]string(nil), items...)
		return nil
	}

	if err := managedRun(cfg); err != nil {
		t.Fatalf("managedRun returned error: %v", err)
	}
	if !reflect.DeepEqual(installs, []string{"AppA", "AppB"}) {
		t.Fatalf("full-run installs = %v", installs)
	}
	if !reflect.DeepEqual(uninstalls, []string{"AppC"}) {
		t.Fatalf("full-run uninstalls = %v", uninstalls)
	}
	if !reflect.DeepEqual(updates, []string{"AppD"}) {
		t.Fatalf("full-run updates = %v", updates)
	}
}

func TestPrepareManagedExecutionIncludesManifestCatalogs(t *testing.T) {
	withManagedExecutionHooks(t)
	cfg := targetedTestConfig(t)
	cfg.Catalogs = []string{"configured"}

	var gotCatalogs []string
	catalogGetFunc = func(cfg config.Configuration) (map[int]map[string]catalog.Item, error) {
		gotCatalogs = append([]string(nil), cfg.Catalogs...)
		return map[int]map[string]catalog.Item{0: {}}, nil
	}

	processInstallResultsFunc = func([]string, map[int]map[string]catalog.Item, string, string, bool) []process.ItemResult {
		return []process.ItemResult{{ItemName: "AppB", Result: installer.Result{Outcome: installer.OutcomeSucceeded}}}
	}

	if _, err := managedItemRun(cfg, "AppB", "InstallItem"); err != nil {
		t.Fatalf("managedItemRun returned error: %v", err)
	}
	if want := []string{"configured", "manifest-catalog"}; !reflect.DeepEqual(gotCatalogs, want) {
		t.Fatalf("effective catalogs = %v, want %v", gotCatalogs, want)
	}
}

func TestManagedItemRunPropagatesPreparationFailure(t *testing.T) {
	withManagedExecutionHooks(t)
	cfg := targetedTestConfig(t)

	manifestGetFunc = func(config.Configuration) ([]manifest.Item, []string, error) {
		return nil, nil, errors.New("manifest boom")
	}

	_, err := managedItemRun(cfg, "AppB", "InstallItem")
	if err == nil || !strings.Contains(err.Error(), "unable to retrieve manifest: manifest boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}
