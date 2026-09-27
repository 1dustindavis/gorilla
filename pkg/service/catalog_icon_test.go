package service

import (
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/manifest"
	"github.com/1dustindavis/gorilla/pkg/status"
)

func TestOptionalIconProjectionIsBestEffortPresentationMetadata(t *testing.T) {
	catalogs := map[int]map[string]catalog.Item{1: {
		"Example": {
			DisplayName: "Example App",
			Icon:        "icons/example.png",
			Installer:   catalog.InstallerItem{Type: "msi", Location: "example.msi"},
			Uninstaller: catalog.InstallerItem{Type: "msi", Location: "example.msi"},
		},
	}}
	observations := map[string]status.Observation{
		"Example App": {State: status.Installed, CheckedAtUTC: time.Now().UTC()},
	}
	stubOptionalCatalog(t, []manifest.Item{{OptionalInstalls: []string{"Example"}}}, catalogs, observations)
	cfg := config.Configuration{URL: "https://repo.example/gorilla/", CachePath: t.TempDir(), AppDataPath: t.TempDir(), Catalogs: []string{"primary"}}

	original := iconResolve
	t.Cleanup(func() { iconResolve = original })
	iconResolve = func(repositoryURL, cachePath, source string) (string, error) {
		if repositoryURL != cfg.URL || cachePath != cfg.CachePath || source != "icons/example.png" {
			t.Fatalf("unexpected resolver input: %q %q %q", repositoryURL, cachePath, source)
		}
		return `C:\ProgramData\Gorilla\cache\catalog-icons\example.png`, nil
	}
	success, err := getOptionalItemDetails(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := success[0].Contract.IconPath; got == "" {
		t.Fatal("resolved icon path was not projected")
	}

	iconResolve = func(string, string, string) (string, error) {
		return "", errors.New("icon unavailable")
	}
	failed, err := getOptionalItemDetails(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if failed[0].Contract.IconPath != "" {
		t.Fatalf("failed icon unexpectedly exposed path %q", failed[0].Contract.IconPath)
	}
	if !reflect.DeepEqual(success[0].Contract.Observation, failed[0].Contract.Observation) {
		t.Fatalf("icon failure changed observation: success=%+v failed=%+v", success[0].Contract.Observation, failed[0].Contract.Observation)
	}
	if !reflect.DeepEqual(success[0].Contract.Policy, failed[0].Contract.Policy) {
		t.Fatalf("icon failure changed policy: success=%+v failed=%+v", success[0].Contract.Policy, failed[0].Contract.Policy)
	}
	if !reflect.DeepEqual(success[0].Contract.Actions, failed[0].Contract.Actions) {
		t.Fatalf("icon failure changed actions: success=%+v failed=%+v", success[0].Contract.Actions, failed[0].Contract.Actions)
	}
}

func TestOptionalIconResolutionUsesBoundedConcurrency(t *testing.T) {
	items := map[string]catalog.Item{}
	optional := make([]string, 0, 12)
	observations := map[string]status.Observation{}
	for i := 0; i < 12; i++ {
		name := fmt.Sprintf("App%02d", i)
		optional = append(optional, name)
		items[name] = catalog.Item{
			DisplayName: name,
			Icon:        fmt.Sprintf("icons/%s.png", name),
			Installer:   catalog.InstallerItem{Type: "msi", Location: name + ".msi"},
		}
		observations[name] = status.Observation{State: status.Absent, CheckedAtUTC: time.Now().UTC()}
	}
	stubOptionalCatalog(t, []manifest.Item{{OptionalInstalls: optional}}, map[int]map[string]catalog.Item{1: items}, observations)

	original := iconResolve
	t.Cleanup(func() { iconResolve = original })
	var active int32
	var maximum int32
	iconResolve = func(string, string, string) (string, error) {
		now := atomic.AddInt32(&active, 1)
		for {
			old := atomic.LoadInt32(&maximum)
			if now <= old || atomic.CompareAndSwapInt32(&maximum, old, now) {
				break
			}
		}
		time.Sleep(15 * time.Millisecond)
		atomic.AddInt32(&active, -1)
		return "cached.png", nil
	}

	details, err := getOptionalItemDetails(config.Configuration{URL: "https://repo.example/", CachePath: t.TempDir(), AppDataPath: t.TempDir(), Catalogs: []string{"primary"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(details) != len(optional) {
		t.Fatalf("got %d details, want %d", len(details), len(optional))
	}
	if maximum > maxConcurrentIconResolutions {
		t.Fatalf("resolver concurrency = %d, limit = %d", maximum, maxConcurrentIconResolutions)
	}
	if maximum < 2 {
		t.Fatalf("resolver ran serially; maximum concurrency = %d", maximum)
	}
}
