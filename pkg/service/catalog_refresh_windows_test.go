//go:build windows

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/manifest"
	"github.com/1dustindavis/gorilla/pkg/status"
)

func TestCatalogRefreshRequestsCoalesceWithoutBlocking(t *testing.T) {
	sr := newServiceRunner(config.Configuration{AppDataPath: t.TempDir()}, nil)
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			sr.requestCatalogRefresh()
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("requestCatalogRefresh blocked")
	}
	sr.catalogRefreshMu.Lock()
	status := sr.catalogRefresh.Status
	sr.catalogRefreshMu.Unlock()
	if status != catalogRefreshQueued {
		t.Fatalf("refresh status = %q, want Queued", status)
	}
	if len(sr.catalogSignal) != 1 {
		t.Fatalf("signal count = %d, want one coalesced signal", len(sr.catalogSignal))
	}
}

func TestCatalogRefreshWaitsForExecutionAndPublishes(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir(), Catalogs: []string{"primary"}}
	stubOptionalCatalog(t,
		[]manifest.Item{{OptionalInstalls: []string{"Example"}}},
		map[int]map[string]catalog.Item{1: {"Example": {DisplayName: "Example", Installer: catalog.InstallerItem{Type: "msi", Location: "example.msi"}}}},
		map[string]status.Observation{"Example": {State: status.Absent, CheckedAtUTC: time.Now().UTC()}},
	)
	sr := newServiceRunner(cfg, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sr.wg.Add(1)
	go func() { defer sr.wg.Done(); sr.catalogRefreshWorker(ctx) }()

	sr.execMutex.Lock()
	sr.requestCatalogRefresh()
	time.Sleep(30 * time.Millisecond)
	if _, ok := sr.currentCatalogSnapshot(); ok {
		t.Fatal("refresh ran while execMutex was held")
	}
	sr.execMutex.Unlock()

	waitForRefreshStatus(t, sr, catalogRefreshIdle)
	snapshot, ok := sr.currentCatalogSnapshot()
	if !ok || len(snapshot.Items) != 1 || snapshot.Items[0].ItemName != "Example" {
		t.Fatalf("refresh did not publish expected snapshot: %#v", snapshot)
	}
	cancel()
	sr.wg.Wait()
}

func TestCatalogRefreshFailureRetainsSnapshotAndLaterRetries(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir(), Catalogs: []string{"primary"}}
	sr := newServiceRunner(cfg, nil)
	sr.catalogSnapshot = testSnapshot(t, cfg, "Old")

	originalManifestGet, originalCatalogGet := manifestGet, catalogGet
	t.Cleanup(func() { manifestGet, catalogGet = originalManifestGet, originalCatalogGet })
	manifestGet = func(config.Configuration) ([]manifest.Item, []string, error) {
		return nil, nil, errors.New("repository unavailable")
	}
	catalogGet = func(config.Configuration) (map[int]map[string]catalog.Item, error) {
		return map[int]map[string]catalog.Item{}, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sr.wg.Add(1)
	go func() { defer sr.wg.Done(); sr.catalogRefreshWorker(ctx) }()
	sr.requestCatalogRefresh()
	waitForRefreshStatus(t, sr, catalogRefreshFailed)
	old, _ := sr.currentCatalogSnapshot()
	if len(old.Items) != 1 || old.Items[0].ItemName != "Old" {
		t.Fatalf("failed refresh replaced last-known-good snapshot: %#v", old)
	}

	manifestGet = func(config.Configuration) ([]manifest.Item, []string, error) {
		return []manifest.Item{}, nil, nil
	}
	sr.requestCatalogRefresh()
	waitForRefreshStatus(t, sr, catalogRefreshIdle)
	fresh, ok := sr.currentCatalogSnapshot()
	if !ok || len(fresh.Items) != 0 {
		t.Fatalf("retry did not publish authoritative empty snapshot: %#v", fresh)
	}
	cancel()
	sr.wg.Wait()
}

func TestCatalogPublicationSatisfiesQueuedRefresh(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir()}
	sr := newServiceRunner(cfg, nil)
	sr.requestCatalogRefresh()
	if err := sr.publishCatalogSnapshot(nil, cfg, catalogSnapshotManagedRun); err != nil {
		t.Fatal(err)
	}
	sr.catalogRefreshMu.Lock()
	state := sr.catalogRefresh
	sr.catalogRefreshMu.Unlock()
	if state.Status != catalogRefreshIdle || state.CompletedAtUTC.Before(state.RequestedAtUTC) {
		t.Fatalf("managed publication did not satisfy queued refresh: %+v", state)
	}

	called := false
	originalManifestGet := manifestGet
	t.Cleanup(func() { manifestGet = originalManifestGet })
	manifestGet = func(config.Configuration) ([]manifest.Item, []string, error) {
		called = true
		return nil, nil, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	sr.wg.Add(1)
	go func() { defer sr.wg.Done(); sr.catalogRefreshWorker(ctx) }()
	waitUntil(t, func() bool { return len(sr.catalogSignal) == 0 })
	cancel()
	sr.wg.Wait()
	if called {
		t.Fatal("satisfied queued refresh performed redundant repository projection")
	}
}

func waitForRefreshStatus(t *testing.T, sr *serviceRunner, want catalogRefreshStatus) {
	t.Helper()
	waitUntil(t, func() bool {
		sr.catalogRefreshMu.Lock()
		defer sr.catalogRefreshMu.Unlock()
		return sr.catalogRefresh.Status == want
	})
}

func waitUntil(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not satisfied before timeout")
}
