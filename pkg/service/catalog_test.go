package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/1dustindavis/gorilla/pkg/appcatalog"
	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/manifest"
	"github.com/1dustindavis/gorilla/pkg/status"
	"go.yaml.in/yaml/v4"
)

func stubOptionalCatalog(t *testing.T, manifests []manifest.Item, catalogs map[int]map[string]catalog.Item, observations map[string]status.Observation) {
	t.Helper()
	originalManifestGet, originalCatalogGet, originalObserve := manifestGet, catalogGet, statusObserve
	t.Cleanup(func() {
		manifestGet, catalogGet, statusObserve = originalManifestGet, originalCatalogGet, originalObserve
	})
	manifestGet = func(config.Configuration) ([]manifest.Item, []string, error) {
		return manifests, []string{"extra"}, nil
	}
	catalogGet = func(config.Configuration) (map[int]map[string]catalog.Item, error) { return catalogs, nil }
	statusObserve = func(item catalog.Item, _, _ string) (status.Observation, error) {
		observation, ok := observations[item.DisplayName]
		if !ok {
			return status.Observation{}, errors.New("missing test observation")
		}
		return observation, nil
	}
}

func TestOptionalDetailsUseCatalogPrecedenceObservationAndPolicy(t *testing.T) {
	now := time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC)
	catalogs := map[int]map[string]catalog.Item{
		1: {"Example": {DisplayName: "Example App", Version: "2.0", Installer: catalog.InstallerItem{Type: "msi", Location: "example.msi"}, Uninstaller: catalog.InstallerItem{Type: "msi", Location: "example.msi"}}},
		2: {"Example": {DisplayName: "Wrong Precedence", Version: "9.0", Installer: catalog.InstallerItem{Type: "exe", Location: "wrong.exe"}}},
	}
	stubOptionalCatalog(t,
		[]manifest.Item{{OptionalInstalls: []string{"Example", "Missing"}}, {Installs: []string{"Example"}}},
		catalogs,
		map[string]status.Observation{"Example App": {State: status.Installed, InstalledVersion: ptr("1.7"), CheckedAtUTC: now}},
	)
	cfg := config.Configuration{AppDataPath: t.TempDir(), Catalogs: []string{"primary"}}
	details, err := getOptionalItemDetails(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(details) != 2 {
		t.Fatalf("got %d items", len(details))
	}
	var missing, example appcatalog.Item
	for _, detail := range details {
		if detail.Contract.ItemName == "Missing" {
			missing = detail.Contract
		} else {
			example = detail.Contract
		}
	}
	if missing.ItemName != "Missing" || missing.Observation.State != appcatalog.Unknown || missing.Observation.DetailCode != "catalog_item_missing_or_invalid" {
		t.Fatalf("unexpected missing item: %+v", missing)
	}
	if example.DisplayName != "Example App" || example.Catalog != "primary" || example.TargetVersion == nil || *example.TargetVersion != "2.0" {
		t.Fatalf("catalog metadata or precedence drifted: %+v", example)
	}
	if example.Observation.State != appcatalog.Installed || example.Observation.InstalledVersion == nil || *example.Observation.InstalledVersion != "1.7" {
		t.Fatalf("observation was not preserved: %+v", example.Observation)
	}
	if !example.Policy.RequiredInstall || example.Actions.Install.Allowed || example.Actions.Install.Reason != "required_install" {
		t.Fatalf("required policy was not enforced: %+v", example)
	}
}

func TestOptionalScriptRequirementAllowsActionsWithoutClaimingPresence(t *testing.T) {
	for _, tc := range []struct {
		name             string
		actionNeeded     bool
		wantRequirement  appcatalog.RequirementState
		wantRemove       bool
		wantRemoveReason string
	}{
		{"requirement not satisfied", true, appcatalog.RequirementNotSatisfied, false, "already_absent"},
		{"requirement satisfied", false, appcatalog.RequirementSatisfied, true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubOptionalCatalog(t,
				[]manifest.Item{{OptionalInstalls: []string{"Scripted"}}},
				map[int]map[string]catalog.Item{1: {"Scripted": {
					DisplayName: "Scripted",
					Check: catalog.InstallCheck{
						Script: "selected-script-check",
						File:   []catalog.FileCheck{{Path: "ignored-lower-priority-check"}},
					},
					Installer:   catalog.InstallerItem{Type: "exe", Location: "scripted.exe"},
					Uninstaller: catalog.InstallerItem{Type: "exe", Location: "remove-scripted.exe"},
				}}},
				map[string]status.Observation{"Scripted": {
					State:        status.Unknown,
					ActionNeeded: tc.actionNeeded,
					CheckedAtUTC: time.Now().UTC(),
				}},
			)

			details, err := getOptionalItemDetails(config.Configuration{AppDataPath: t.TempDir(), Catalogs: []string{"primary"}})
			if err != nil {
				t.Fatal(err)
			}
			item := details[0].Contract
			if item.Observation.State != appcatalog.Unknown || item.Observation.InstallRequirement != tc.wantRequirement {
				t.Fatalf("script requirement was converted into physical presence: %+v", item.Observation)
			}
			if !item.Actions.Install.Allowed || item.Actions.Remove.Allowed != tc.wantRemove || item.Actions.Remove.Reason != tc.wantRemoveReason {
				t.Fatalf("unexpected script actions: %+v", item.Actions)
			}
		})
	}
}

func TestInstallRejectsNonOptionalBeforeMutation(t *testing.T) {
	stubOptionalCatalog(t, []manifest.Item{{OptionalInstalls: []string{"Allowed"}}}, map[int]map[string]catalog.Item{}, map[string]status.Observation{})
	cfg := config.Configuration{AppDataPath: t.TempDir(), Catalogs: []string{"primary"}}
	_, err := executeCommand(cfg, Command{Action: actionInstallItem, Items: []string{"Other"}}, func(config.Configuration) error { return nil })
	var denied actionDeniedError
	if !errors.As(err, &denied) || denied.reason != "not_optional" {
		t.Fatalf("got %v", err)
	}
	if _, statErr := os.Stat(serviceLocalManifestPath(cfg)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("mutation occurred: %v", statErr)
	}
}

func TestCatalogLoadFailureKeepsOptionalEntryUnavailable(t *testing.T) {
	originalManifestGet, originalCatalogGet := manifestGet, catalogGet
	t.Cleanup(func() { manifestGet, catalogGet = originalManifestGet, originalCatalogGet })
	manifestGet = func(config.Configuration) ([]manifest.Item, []string, error) {
		return []manifest.Item{{OptionalInstalls: []string{"Example"}}}, nil, nil
	}
	catalogGet = func(config.Configuration) (map[int]map[string]catalog.Item, error) {
		return nil, errors.New("catalog unavailable")
	}
	details, err := getOptionalItemDetails(config.Configuration{AppDataPath: t.TempDir(), Catalogs: []string{"primary"}})
	if err != nil {
		t.Fatal(err)
	}
	item := details[0].Contract
	if item.Observation.State != appcatalog.Unknown || item.Observation.DetailCode != "catalog_load_failed" || item.Actions.Install.Allowed || item.Actions.Remove.Allowed {
		t.Fatalf("catalog failure was not represented safely: %+v", item)
	}
}

func TestAdministratorPolicyStillWinsWhenServiceSelectionMatches(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir(), Catalogs: []string{"primary"}}
	cfg.LocalManifests = []string{serviceLocalManifestPath(cfg)}
	selection := manifest.Item{Name: "service-manifest", Installs: []string{"Required", "Forbidden"}}
	if err := saveServiceLocalManifest(cfg, selection); err != nil {
		t.Fatal(err)
	}
	entries := []manifest.Item{
		{Name: "admin", OptionalInstalls: []string{"Required", "Forbidden"}, Installs: []string{"Required"}, Uninstalls: []string{"Forbidden"}},
		selection,
	}
	items := map[int]map[string]catalog.Item{1: {
		"Required":  {DisplayName: "Required", Installer: catalog.InstallerItem{Type: "msi", Location: "required.msi"}},
		"Forbidden": {DisplayName: "Forbidden", Installer: catalog.InstallerItem{Type: "msi", Location: "forbidden.msi"}},
	}}
	stubOptionalCatalog(t, entries, items, map[string]status.Observation{
		"Required":  {State: status.Installed, CheckedAtUTC: time.Now().UTC()},
		"Forbidden": {State: status.Absent, CheckedAtUTC: time.Now().UTC()},
	})
	details, err := getOptionalItemDetails(cfg)
	if err != nil {
		t.Fatal(err)
	}
	required, _ := findOptionalItem(details, "Required")
	forbidden, _ := findOptionalItem(details, "Forbidden")
	if !required.Contract.Policy.RequiredInstall || required.Contract.Actions.Remove.Reason != "required_install" {
		t.Fatalf("administrator install was subtracted: %+v", required.Contract.Policy)
	}
	if !forbidden.Contract.Policy.RequiredUninstall || forbidden.Contract.Actions.Install.Reason != "managed_uninstall" {
		t.Fatalf("administrator uninstall was subtracted: %+v", forbidden.Contract.Policy)
	}
	if forbidden.Contract.Policy.Selection != appcatalog.NoSelection {
		t.Fatalf("conflicting local selection was not suppressed: %+v", forbidden.Contract.Policy)
	}
	persisted, err := loadServiceLocalManifest(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(persisted.Installs, []string{"Required"}) {
		t.Fatalf("conflicting local selection was not removed: %v", persisted.Installs)
	}
}

func TestScheduledRunRemovesSelectionOverriddenByAdministratorUninstall(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir()}
	cfg.LocalManifests = []string{serviceLocalManifestPath(cfg)}
	selection := manifest.Item{Name: "service-manifest", Installs: []string{"Conflict", "Keep"}}
	if err := saveServiceLocalManifest(cfg, selection); err != nil {
		t.Fatal(err)
	}

	originalManifestGet := manifestGet
	t.Cleanup(func() { manifestGet = originalManifestGet })
	manifestGet = func(config.Configuration) ([]manifest.Item, []string, error) {
		return []manifest.Item{
			{Name: "administrator", Uninstalls: []string{"Conflict"}},
			selection,
		}, nil, nil
	}

	managedRunCalled := false
	_, err := executeCommand(cfg, Command{Action: actionRun}, func(config.Configuration) error {
		managedRunCalled = true
		got, loadErr := loadServiceLocalManifest(cfg)
		if loadErr != nil {
			return loadErr
		}
		if !slices.Equal(got.Installs, []string{"Keep"}) {
			return fmt.Errorf("managed run saw unreconciled selections: %v", got.Installs)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !managedRunCalled {
		t.Fatal("scheduled managed run was not called")
	}
}

func TestInstallRejectsUnsatisfiableDependencyGraphs(t *testing.T) {
	valid := func(name string, dependencies ...string) catalog.Item {
		return catalog.Item{
			DisplayName:  name,
			Dependencies: dependencies,
			Installer:    catalog.InstallerItem{Type: "exe", Location: name + ".exe"},
		}
	}

	for _, tc := range []struct {
		name      string
		manifests []manifest.Item
		catalogs  map[int]map[string]catalog.Item
		allowed   bool
	}{
		{
			name:      "valid nested dependencies",
			manifests: []manifest.Item{{OptionalInstalls: []string{"Parent"}}},
			catalogs: map[int]map[string]catalog.Item{1: {
				"Parent":     valid("Parent", "Child"),
				"Child":      valid("Child", "Grandchild"),
				"Grandchild": valid("Grandchild"),
			}},
			allowed: true,
		},
		{
			name:      "missing dependency",
			manifests: []manifest.Item{{OptionalInstalls: []string{"Parent"}}},
			catalogs:  map[int]map[string]catalog.Item{1: {"Parent": valid("Parent", "Missing")}},
		},
		{
			name:      "invalid dependency",
			manifests: []manifest.Item{{OptionalInstalls: []string{"Parent"}}},
			catalogs: map[int]map[string]catalog.Item{1: {
				"Parent":  valid("Parent", "Invalid"),
				"Invalid": {DisplayName: "Invalid"},
			}},
		},
		{
			name:      "dependency cycle",
			manifests: []manifest.Item{{OptionalInstalls: []string{"Parent"}}},
			catalogs: map[int]map[string]catalog.Item{1: {
				"Parent": valid("Parent", "Child"),
				"Child":  valid("Child", "Parent"),
			}},
		},
		{
			name: "dependency forced absent",
			manifests: []manifest.Item{{
				OptionalInstalls: []string{"Parent"},
				Uninstalls:       []string{"Child"},
			}},
			catalogs: map[int]map[string]catalog.Item{1: {
				"Parent": valid("Parent", "Child"),
				"Child":  valid("Child"),
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubOptionalCatalog(t, tc.manifests, tc.catalogs, map[string]status.Observation{
				"Parent": {State: status.Absent, ActionNeeded: true, CheckedAtUTC: time.Now().UTC()},
			})
			details, err := getOptionalItemDetails(config.Configuration{AppDataPath: t.TempDir(), Catalogs: []string{"primary"}})
			if err != nil {
				t.Fatal(err)
			}
			parent := details[0].Contract
			if parent.Actions.Install.Allowed != tc.allowed {
				t.Fatalf("dependency feasibility produced wrong install decision: %+v", parent.Actions.Install)
			}
			if !tc.allowed && parent.Actions.Install.Reason != "install_unavailable" {
				t.Fatalf("unsatisfiable dependency graph used wrong reason: %+v", parent.Actions.Install)
			}
		})
	}
}

func TestInstallRejectsUnsatisfiableDependencyBeforePersistingSelection(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir(), Catalogs: []string{"primary"}}
	stubOptionalCatalog(t,
		[]manifest.Item{{OptionalInstalls: []string{"Parent"}}},
		map[int]map[string]catalog.Item{1: {"Parent": {
			DisplayName:  "Parent",
			Dependencies: []string{"Missing"},
			Installer:    catalog.InstallerItem{Type: "exe", Location: "parent.exe"},
		}}},
		map[string]status.Observation{"Parent": {
			State: status.Absent, ActionNeeded: true, CheckedAtUTC: time.Now().UTC(),
		}},
	)

	_, err := executeCommand(cfg, Command{Action: actionInstallItem, Items: []string{"Parent"}}, func(config.Configuration) error { return nil })
	var denied actionDeniedError
	if !errors.As(err, &denied) || denied.reason != "install_unavailable" {
		t.Fatalf("got %v", err)
	}
	if _, statErr := os.Stat(serviceLocalManifestPath(cfg)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("dependency failure persisted a selection: %v", statErr)
	}
}

func TestOneTimeRemovalClearsSelectionWithoutPersistingUninstall(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir()}
	if err := addServiceManagedInstalls(cfg, []string{"Example"}); err != nil {
		t.Fatal(err)
	}
	runCfg, cleanup, err := prepareOneTimeRemoval(cfg, "Example", "op-1", true)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := loadServiceLocalManifest(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(entry.Installs) != 0 || len(entry.Uninstalls) != 0 {
		t.Fatalf("persistent policy remains: %+v", entry)
	}
	if len(runCfg.LocalManifests) != 1 || runCfg.LocalManifests[0] != cleanup {
		t.Fatalf("one-time manifest not scoped to run: %+v", runCfg.LocalManifests)
	}
	data, err := os.ReadFile(cleanup)
	if err != nil {
		t.Fatal(err)
	}
	var oneTime manifest.Item
	if err := yaml.Unmarshal(data, &oneTime); err != nil {
		t.Fatal(err)
	}
	if len(oneTime.Uninstalls) != 1 || oneTime.Uninstalls[0] != "Example" {
		t.Fatalf("bad one-time manifest: %+v", oneTime)
	}
	if filepath.Dir(cleanup) != filepath.Join(cfg.AppDataPath, "operations") {
		t.Fatalf("unexpected path: %s", cleanup)
	}
}

func TestExecuteRemoveReturnsOneTimeRunConfiguration(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir(), Catalogs: []string{"primary"}}
	if err := addServiceManagedInstalls(cfg, []string{"Example"}); err != nil {
		t.Fatal(err)
	}
	stubOptionalCatalog(t,
		[]manifest.Item{{OptionalInstalls: []string{"Example"}}},
		map[int]map[string]catalog.Item{1: {"Example": {
			DisplayName: "Example", Installer: catalog.InstallerItem{Type: "msi", Location: "example.msi"},
			Uninstaller: catalog.InstallerItem{Type: "msi", Location: "example.msi"},
		}}},
		map[string]status.Observation{"Example": {State: status.Installed, CheckedAtUTC: time.Now().UTC()}},
	)
	resp, err := executeCommand(cfg, Command{Action: actionRemoveItem, Items: []string{"Example"}}, func(config.Configuration) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if resp.RunConfig == nil || resp.CleanupPath == "" {
		t.Fatalf("one-time removal was not prepared: %+v", resp)
	}
	entry, err := loadServiceLocalManifest(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(entry.Installs) != 0 || len(entry.Uninstalls) != 0 {
		t.Fatalf("removal persisted policy: %+v", entry)
	}
}

func TestLegacyServiceUninstallsAreCleared(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir()}
	if err := saveServiceLocalManifest(cfg, manifest.Item{Name: "service-manifest", Uninstalls: []string{"OldRemoval"}}); err != nil {
		t.Fatal(err)
	}
	if err := clearLegacyServiceUninstalls(cfg); err != nil {
		t.Fatal(err)
	}
	entry, err := loadServiceLocalManifest(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(entry.Uninstalls) != 0 {
		t.Fatalf("legacy removal remains: %+v", entry)
	}
}

func ptr(value string) *string { return &value }
