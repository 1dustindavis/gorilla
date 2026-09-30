package main

import (
	"reflect"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/installer"
	"github.com/1dustindavis/gorilla/pkg/process"
)

func TestManagedItemRunRemoveReturnsExecutionScopedPreparedContext(t *testing.T) {
	withManagedExecutionHooks(t)
	cfg := targetedTestConfig(t)
	cfg.LocalManifests = []string{"operation-scoped-remove.yaml"}

	processUninstallResultsFunc = func(uninstalls []string, _ map[int]map[string]catalog.Item, _, _ string, _ bool) []process.ItemResult {
		return []process.ItemResult{{
			ItemName: "AppB",
			Result: installer.Result{
				ItemName: "AppB",
				Action:   "uninstall",
				Outcome:  installer.OutcomeSucceeded,
			},
		}}
	}

	result, err := managedItemRun(cfg, "AppB", "RemoveItem")
	if err != nil {
		t.Fatalf("managedItemRun returned error: %v", err)
	}
	if !reflect.DeepEqual(result.ExecutionPrepared.Config.LocalManifests, cfg.LocalManifests) {
		t.Fatalf(
			"execution prepared local manifests = %v, want transient execution config %v",
			result.ExecutionPrepared.Config.LocalManifests,
			cfg.LocalManifests,
		)
	}
}
