package service

import (
	"errors"
	"os"
	"path/filepath"
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
	selection := manifest.Item{Name: "service-manifest", Installs: []string{"Required"}, Uninstalls: []string{"Forbidden"}}
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
