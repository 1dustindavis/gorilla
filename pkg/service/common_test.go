package service

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/config"
)

func TestParseCommandSpecInstallItem(t *testing.T) {
	cmd, err := parseCommandSpec("InstallItem:GoogleChrome")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cmd.Action != actionInstallItem {
		t.Fatalf("expected action %s, got %s", actionInstallItem, cmd.Action)
	}
	if len(cmd.Items) != 1 || cmd.Items[0] != "GoogleChrome" {
		t.Fatalf("unexpected items: %#v", cmd.Items)
	}
}

func TestParseCommandSpecRemoveItem(t *testing.T) {
	cmd, err := parseCommandSpec("RemoveItem:GoogleChrome")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cmd.Action != actionRemoveItem {
		t.Fatalf("expected action %s, got %s", actionRemoveItem, cmd.Action)
	}
	if len(cmd.Items) != 1 || cmd.Items[0] != "GoogleChrome" {
		t.Fatalf("unexpected items: %#v", cmd.Items)
	}
}

func TestParseCommandSpecListOptionalInstalls(t *testing.T) {
	cmd, err := parseCommandSpec("ListOptionalInstalls")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cmd.Action != actionListOptionalInstalls {
		t.Fatalf("expected action %s, got %s", actionListOptionalInstalls, cmd.Action)
	}
	if len(cmd.Items) != 0 {
		t.Fatalf("expected no items, got %#v", cmd.Items)
	}
}

func TestParseCommandSpecStreamOperationStatus(t *testing.T) {
	cmd, err := parseCommandSpec("StreamOperationStatus:op-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cmd.Action != actionStreamOperationStatus {
		t.Fatalf("expected action %s, got %s", actionStreamOperationStatus, cmd.Action)
	}
	if len(cmd.Items) != 1 || cmd.Items[0] != "op-123" {
		t.Fatalf("unexpected items: %#v", cmd.Items)
	}
}

func TestParseCommandSpecInvalid(t *testing.T) {
	_, err := parseCommandSpec("InstallItem")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseCommandSpecLegacyActionInvalid(t *testing.T) {
	_, err := parseCommandSpec("install:foo")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateCommandRunWithItems(t *testing.T) {
	err := validateCommand(Command{
		Action: actionRun,
		Items:  []string{"foo"},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateCommandInstallItemRequiresOneArgument(t *testing.T) {
	err := validateCommand(Command{
		Action: actionInstallItem,
		Items:  []string{"foo", "bar"},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestServiceInstallArgs(t *testing.T) {
	configPath := `C:\ProgramData\gorilla\config.yaml`
	got := serviceInstallArgs(configPath, "")
	if len(got) != 3 {
		t.Fatalf("expected 3 args, got %d: %#v", len(got), got)
	}
	if got[0] != "-c" {
		t.Fatalf("expected first arg -c, got %q", got[0])
	}
	if got[1] != configPath {
		t.Fatalf("expected config path %q, got %q", configPath, got[1])
	}
	if got[2] != "-service" {
		t.Fatalf("expected final arg -service, got %q", got[2])
	}
}

func TestServiceInstallArgsPreservesIntegrationTestIdentity(t *testing.T) {
	configPath := `C:\ProgramData\gorilla-ui-e2e\config.yaml`
	got := serviceInstallArgs(configPath, "gorilla-ui-e2e")
	want := []string{
		"-c", configPath, "-service",
		"-integration-test-service-identity", "gorilla-ui-e2e",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestServiceManagedInstallAndRemovePersistDesiredState(t *testing.T) {
	cfg := config.Configuration{AppDataPath: t.TempDir()}

	if err := addServiceManagedInstalls(cfg, []string{"Example"}); err != nil {
		t.Fatalf("add managed install: %v", err)
	}
	entry, err := loadServiceLocalManifest(cfg)
	if err != nil {
		t.Fatalf("load after install: %v", err)
	}
	if !slices.Equal(entry.Installs, []string{"Example"}) || len(entry.Uninstalls) != 0 {
		t.Fatalf("unexpected install desired state: installs=%v uninstalls=%v", entry.Installs, entry.Uninstalls)
	}

	if err := removeServiceManagedInstalls(cfg, []string{"Example"}); err != nil {
		t.Fatalf("remove managed install: %v", err)
	}
	entry, err = loadServiceLocalManifest(cfg)
	if err != nil {
		t.Fatalf("load after remove: %v", err)
	}
	if len(entry.Installs) != 0 || !slices.Equal(entry.Uninstalls, []string{"Example"}) {
		t.Fatalf("unexpected remove desired state: installs=%v uninstalls=%v", entry.Installs, entry.Uninstalls)
	}

	if err := addServiceManagedInstalls(cfg, []string{"Example"}); err != nil {
		t.Fatalf("re-add managed install: %v", err)
	}
	entry, err = loadServiceLocalManifest(cfg)
	if err != nil {
		t.Fatalf("load after re-add: %v", err)
	}
	if !slices.Equal(entry.Installs, []string{"Example"}) || len(entry.Uninstalls) != 0 {
		t.Fatalf("unexpected re-install desired state: installs=%v uninstalls=%v", entry.Installs, entry.Uninstalls)
	}

	if filepath.Base(serviceLocalManifestPath(cfg)) != "service-manifest.yaml" {
		t.Fatalf("unexpected service manifest path: %s", serviceLocalManifestPath(cfg))
	}
}
