//go:build windows

package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
)

const (
	optionalCatalogSnapshotSchemaVersion = 1
	optionalCatalogSnapshotFilename      = "app-catalog-snapshot.json"
)

type catalogSnapshotTrigger string

const (
	catalogSnapshotManagedRun        catalogSnapshotTrigger = "managed-run"
	catalogSnapshotTargetedInstall   catalogSnapshotTrigger = "targeted-install"
	catalogSnapshotTargetedRemove    catalogSnapshotTrigger = "targeted-remove"
	catalogSnapshotBackgroundRefresh catalogSnapshotTrigger = "background-refresh"
)

type optionalCatalogSnapshot struct {
	SchemaVersion     int                           `json:"schemaVersion"`
	SourceFingerprint string                        `json:"sourceFingerprint"`
	GeneratedAtUTC    time.Time                     `json:"generatedAtUtc"`
	Items             []optionalInstallResponseItem `json:"items"`
}

type catalogSnapshotSourceIdentity struct {
	Manifest       string   `json:"manifest"`
	Catalogs       []string `json:"catalogs"`
	LocalManifests []string `json:"localManifests"`
	Repository     string   `json:"repository"`
}

func catalogSnapshotPath(cfg config.Configuration) string {
	return filepath.Join(cfg.AppDataPath, optionalCatalogSnapshotFilename)
}

func catalogSnapshotSourceFingerprint(cfg config.Configuration) (string, error) {
	localManifests := make([]string, len(cfg.LocalManifests))
	for i, path := range cfg.LocalManifests {
		localManifests[i] = filepath.Clean(path)
	}
	identity := catalogSnapshotSourceIdentity{
		Manifest:       cfg.Manifest,
		Catalogs:       append([]string(nil), cfg.Catalogs...),
		LocalManifests: localManifests,
		Repository:     safeRepositoryIdentity(cfg),
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func safeRepositoryIdentity(cfg config.Configuration) string {
	if strings.TrimSpace(cfg.URL) != "" {
		parsed, err := url.Parse(cfg.URL)
		if err == nil {
			parsed.User = nil
			parsed.RawQuery = ""
			parsed.Fragment = ""
			return parsed.String()
		}
		return "invalid-url"
	}
	if strings.TrimSpace(cfg.RepoPath) != "" {
		return filepath.Clean(cfg.RepoPath)
	}
	return ""
}

func cloneCatalogSnapshot(snapshot *optionalCatalogSnapshot) *optionalCatalogSnapshot {
	if snapshot == nil {
		return nil
	}
	copySnapshot := *snapshot
	copySnapshot.Items = append([]optionalInstallResponseItem(nil), snapshot.Items...)
	return &copySnapshot
}

func (sr *serviceRunner) currentCatalogSnapshot() (*optionalCatalogSnapshot, bool) {
	sr.catalogMu.RLock()
	defer sr.catalogMu.RUnlock()
	if sr.catalogSnapshot == nil {
		return nil, false
	}
	return cloneCatalogSnapshot(sr.catalogSnapshot), true
}

func (sr *serviceRunner) publishCatalogSnapshot(details []optionalItemDetails, sourceCfg config.Configuration, trigger catalogSnapshotTrigger) error {
	started := time.Now()
	generatedAt := time.Now().UTC()
	gorillalog.Debug("catalog projection started:", "trigger=", trigger)
	fingerprint, err := catalogSnapshotSourceFingerprint(sourceCfg)
	if err != nil {
		gorillalog.Warn("catalog projection failed:", "trigger=", trigger, "error=", err)
		return err
	}
	candidate := &optionalCatalogSnapshot{
		SchemaVersion:     optionalCatalogSnapshotSchemaVersion,
		SourceFingerprint: fingerprint,
		GeneratedAtUTC:    generatedAt,
		Items:             optionalInstallResponseItems(details, generatedAt),
	}

	// Publish only after the complete candidate exists. Readers can therefore
	// never observe a partially generated snapshot.
	sr.catalogMu.Lock()
	sr.catalogSnapshot = candidate
	sr.catalogMu.Unlock()
	sr.noteCatalogPublication(generatedAt)
	gorillalog.Debug("catalog projection completed:", "trigger=", trigger, "itemCount=", len(candidate.Items), "generatedAt=", generatedAt.Format(time.RFC3339), "durationMs=", time.Since(started).Milliseconds())
	gorillalog.Debug("catalog snapshot published:", "trigger=", trigger, "itemCount=", len(candidate.Items), "generatedAt=", generatedAt.Format(time.RFC3339))

	if err := persistCatalogSnapshot(sr.cfg, candidate); err != nil {
		// Persistence is best effort after publication. The new in-memory snapshot
		// remains authoritative for this process and the old file is left intact
		// whenever replacement never completed.
		gorillalog.Warn("catalog snapshot persistence failed:", "trigger=", trigger, "error=", err)
		return err
	}
	gorillalog.Debug("catalog snapshot persisted:", "trigger=", trigger, "itemCount=", len(candidate.Items), "generatedAt=", generatedAt.Format(time.RFC3339))
	return nil
}

func persistCatalogSnapshot(cfg config.Configuration, snapshot *optionalCatalogSnapshot) error {
	path := catalogSnapshotPath(cfg)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create snapshot directory: %w", err)
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), optionalCatalogSnapshotFilename+"-*.tmp")
	if err != nil {
		return fmt.Errorf("create snapshot candidate: %w", err)
	}
	tmpPath := tmp.Name()
	replaced := false
	defer func() {
		_ = tmp.Close()
		if !replaced {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write snapshot candidate: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("flush snapshot candidate: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close snapshot candidate: %w", err)
	}
	if err := replaceSnapshotFile(tmpPath, path); err != nil {
		return fmt.Errorf("replace snapshot: %w", err)
	}
	replaced = true
	return nil
}

func loadCatalogSnapshot(cfg config.Configuration) (*optionalCatalogSnapshot, error) {
	path := catalogSnapshotPath(cfg)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var snapshot optionalCatalogSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("corrupt snapshot: %w", err)
	}
	if snapshot.SchemaVersion != optionalCatalogSnapshotSchemaVersion {
		return nil, fmt.Errorf("unsupported snapshot schema %d", snapshot.SchemaVersion)
	}
	expected, err := catalogSnapshotSourceFingerprint(cfg)
	if err != nil {
		return nil, err
	}
	if snapshot.SourceFingerprint != expected {
		return nil, errors.New("snapshot source fingerprint mismatch")
	}
	if snapshot.Items == nil {
		snapshot.Items = []optionalInstallResponseItem{}
	}
	return &snapshot, nil
}

func (sr *serviceRunner) loadPersistedCatalogSnapshot() {
	snapshot, err := loadCatalogSnapshot(sr.cfg)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			gorillalog.Debug("catalog snapshot missing")
			return
		}
		message := err.Error()
		switch {
		case strings.Contains(message, "schema"):
			gorillalog.Warn("catalog snapshot ignored: invalid schema")
		case strings.Contains(message, "fingerprint"):
			gorillalog.Warn("catalog snapshot ignored: source mismatch")
		default:
			gorillalog.Warn("catalog snapshot ignored: corrupt")
		}
		return
	}
	sr.catalogMu.Lock()
	sr.catalogSnapshot = snapshot
	sr.catalogMu.Unlock()
	gorillalog.Debug("catalog snapshot loaded:", "itemCount=", len(snapshot.Items), "generatedAt=", snapshot.GeneratedAtUTC.Format(time.RFC3339))
}
