//go:build windows

package service

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/1dustindavis/gorilla/pkg/config"
)

func testSnapshot(t *testing.T, cfg config.Configuration, name string) *optionalCatalogSnapshot {
	t.Helper()
	fingerprint, err := catalogSnapshotSourceFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return &optionalCatalogSnapshot{
		SchemaVersion:     optionalCatalogSnapshotSchemaVersion,
		SourceFingerprint: fingerprint,
		GeneratedAtUTC:    time.Date(2026, 9, 29, 22, 0, 0, 0, time.UTC),
		Items:             []optionalInstallResponseItem{{ItemName: name, DisplayName: name}},
	}
}

func TestCatalogSnapshotPersistLoadAndOverwrite(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir(), Manifest: "site", Catalogs: []string{"primary"}}
	first := testSnapshot(t, cfg, "First")
	if err := persistCatalogSnapshot(cfg, first); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadCatalogSnapshot(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Items) != 1 || loaded.Items[0].ItemName != "First" {
		t.Fatalf("unexpected loaded snapshot: %+v", loaded)
	}

	second := testSnapshot(t, cfg, "Second")
	if err := persistCatalogSnapshot(cfg, second); err != nil {
		t.Fatal(err)
	}
	loaded, err = loadCatalogSnapshot(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Items) != 1 || loaded.Items[0].ItemName != "Second" {
		t.Fatalf("snapshot overwrite failed: %+v", loaded)
	}
}

func TestCatalogSnapshotLoadValidation(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir(), Manifest: "site", Catalogs: []string{"primary"}}
	if _, err := loadCatalogSnapshot(cfg); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing snapshot error = %v, want os.ErrNotExist", err)
	}

	path := catalogSnapshotPath(cfg)
	if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCatalogSnapshot(cfg); err == nil {
		t.Fatal("malformed snapshot was accepted")
	}

	badSchema := testSnapshot(t, cfg, "Example")
	badSchema.SchemaVersion++
	writeSnapshotFixture(t, path, badSchema)
	if _, err := loadCatalogSnapshot(cfg); err == nil {
		t.Fatal("unsupported schema was accepted")
	}

	missingGeneratedAt := testSnapshot(t, cfg, "Example")
	missingGeneratedAt.GeneratedAtUTC = time.Time{}
	writeSnapshotFixture(t, path, missingGeneratedAt)
	if _, err := loadCatalogSnapshot(cfg); err == nil {
		t.Fatal("snapshot with missing generatedAtUtc was accepted")
	}

	mismatch := testSnapshot(t, cfg, "Example")
	mismatch.SourceFingerprint = "wrong"
	writeSnapshotFixture(t, path, mismatch)
	if _, err := loadCatalogSnapshot(cfg); err == nil {
		t.Fatal("source fingerprint mismatch was accepted")
	}

	empty := testSnapshot(t, cfg, "Example")
	empty.Items = []optionalInstallResponseItem{}
	writeSnapshotFixture(t, path, empty)
	loaded, err := loadCatalogSnapshot(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Items == nil || len(loaded.Items) != 0 {
		t.Fatalf("authoritative empty snapshot not preserved: %#v", loaded.Items)
	}
}

func TestCatalogSnapshotFingerprintIsSafeAndOrderSensitive(t *testing.T) {
	base := config.Configuration{
		Manifest:       "site",
		Catalogs:       []string{"first", "second"},
		LocalManifests: []string{filepath.Join("a", "..", "b", "manifest.yaml")},
		URL:            "https://user:secret@example.test/repo?token=secret#fragment",
		AuthUser:       "another-user",
		AuthPass:       "another-secret",
	}
	sanitized := base
	sanitized.URL = "https://example.test/repo"
	sanitized.AuthUser = "different"
	sanitized.AuthPass = "different"
	got, err := catalogSnapshotSourceFingerprint(base)
	if err != nil {
		t.Fatal(err)
	}
	want, err := catalogSnapshotSourceFingerprint(sanitized)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("credentials or URL query affected fingerprint: %q != %q", got, want)
	}

	reordered := sanitized
	reordered.Catalogs = []string{"second", "first"}
	other, err := catalogSnapshotSourceFingerprint(reordered)
	if err != nil {
		t.Fatal(err)
	}
	if other == want {
		t.Fatal("catalog precedence order did not affect fingerprint")
	}
}

func TestCatalogSnapshotPersistenceFailureKeepsPublishedMemory(t *testing.T) {
	root := t.TempDir()
	appDataFile := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(appDataFile, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Configuration{AppDataPath: appDataFile}
	sr := newServiceRunner(cfg, nil)
	details := []optionalItemDetails{{Contract: testAppCatalogItem("Example")}}
	if err := sr.publishCatalogSnapshot(details, cfg, catalogSnapshotManagedRun); err == nil {
		t.Fatal("expected persistence failure")
	}
	snapshot, ok := sr.currentCatalogSnapshot()
	if !ok || len(snapshot.Items) != 1 || snapshot.Items[0].ItemName != "Example" {
		t.Fatalf("in-memory snapshot was lost after persistence failure: %#v", snapshot)
	}
}

func TestCurrentCatalogSnapshotReturnsDefensiveCopy(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir()}
	sr := newServiceRunner(cfg, nil)
	snapshot := testSnapshot(t, cfg, "Example")
	targetVersion := "2.0"
	installedVersion := "1.0"
	checkedAt := time.Date(2026, 9, 29, 22, 1, 0, 0, time.UTC)
	snapshot.Items[0].TargetVersion = &targetVersion
	snapshot.Items[0].Observation.InstalledVersion = &installedVersion
	snapshot.Items[0].Observation.CheckedAtUTC = &checkedAt
	sr.catalogSnapshot = snapshot

	copySnapshot, ok := sr.currentCatalogSnapshot()
	if !ok {
		t.Fatal("missing snapshot")
	}
	copySnapshot.Items[0].ItemName = "Mutated"
	*copySnapshot.Items[0].TargetVersion = "9.0"
	*copySnapshot.Items[0].Observation.InstalledVersion = "8.0"
	mutatedCheckedAt := copySnapshot.Items[0].Observation.CheckedAtUTC.Add(time.Hour)
	*copySnapshot.Items[0].Observation.CheckedAtUTC = mutatedCheckedAt

	again, _ := sr.currentCatalogSnapshot()
	if again.Items[0].ItemName != "Example" {
		t.Fatalf("snapshot item slice was mutable through accessor: %+v", again.Items[0])
	}
	if again.Items[0].TargetVersion == nil || *again.Items[0].TargetVersion != "2.0" {
		t.Fatalf("target version pointer aliased service snapshot: %+v", again.Items[0].TargetVersion)
	}
	if again.Items[0].Observation.InstalledVersion == nil || *again.Items[0].Observation.InstalledVersion != "1.0" {
		t.Fatalf("installed version pointer aliased service snapshot: %+v", again.Items[0].Observation.InstalledVersion)
	}
	if again.Items[0].Observation.CheckedAtUTC == nil || !again.Items[0].Observation.CheckedAtUTC.Equal(checkedAt) {
		t.Fatalf("checked-at pointer aliased service snapshot: %+v", again.Items[0].Observation.CheckedAtUTC)
	}
}

func writeSnapshotFixture(t *testing.T, path string, snapshot *optionalCatalogSnapshot) {
	t.Helper()
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
