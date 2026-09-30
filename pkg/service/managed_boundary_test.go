package service

import (
	"errors"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/managedrun"
	"github.com/1dustindavis/gorilla/pkg/manifest"
)

func TestExecuteCommandRunIgnoresPreparedResult(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir()}
	prepared := managedrun.PreparedContext{
		Config:    config.Configuration{Catalogs: []string{"deliberately-different"}},
		Manifests: []manifest.Item{{Name: "dummy"}},
		Catalogs:  map[int]map[string]catalog.Item{99: {}},
	}

	resp, err := executeCommand(cfg, Command{Action: actionRun}, func(got config.Configuration) (managedrun.RunResult, error) {
		if got.AppDataPath != cfg.AppDataPath {
			t.Fatalf("managed run received AppDataPath %q, want %q", got.AppDataPath, cfg.AppDataPath)
		}
		return managedrun.RunResult{Prepared: prepared}, nil
	})
	if err != nil {
		t.Fatalf("executeCommand(run) returned error: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status = %q, want ok", resp.Status)
	}
}

func TestExecuteCommandRunPropagatesManagedRunError(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir()}
	boom := errors.New("boom")

	resp, err := executeCommand(cfg, Command{Action: actionRun}, func(config.Configuration) (managedrun.RunResult, error) {
		return managedrun.RunResult{}, boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want %v", err, boom)
	}
	if resp.Status != "ok" {
		t.Fatalf("status = %q, want existing ok response", resp.Status)
	}
}
