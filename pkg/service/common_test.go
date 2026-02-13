package service

import "testing"

func TestParseCommandSpecInstall(t *testing.T) {
	cmd, err := parseCommandSpec("install:GoogleChrome, 7zip")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cmd.Action != "install" {
		t.Fatalf("expected action install, got %s", cmd.Action)
	}
	if len(cmd.Items) != 2 || cmd.Items[0] != "GoogleChrome" || cmd.Items[1] != "7zip" {
		t.Fatalf("unexpected items: %#v", cmd.Items)
	}
}

func TestParseCommandSpecRemove(t *testing.T) {
	cmd, err := parseCommandSpec("remove:GoogleChrome")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cmd.Action != "remove" {
		t.Fatalf("expected action remove, got %s", cmd.Action)
	}
	if len(cmd.Items) != 1 || cmd.Items[0] != "GoogleChrome" {
		t.Fatalf("unexpected items: %#v", cmd.Items)
	}
}

func TestParseCommandSpecRun(t *testing.T) {
	cmd, err := parseCommandSpec("run")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cmd.Action != "run" {
		t.Fatalf("expected action run, got %s", cmd.Action)
	}
	if len(cmd.Items) != 0 {
		t.Fatalf("expected no items, got %#v", cmd.Items)
	}
}

func TestParseCommandSpecGetServiceManifest(t *testing.T) {
	cmd, err := parseCommandSpec("get-service-manifest")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cmd.Action != "get-service-manifest" {
		t.Fatalf("expected action get-service-manifest, got %s", cmd.Action)
	}
	if len(cmd.Items) != 0 {
		t.Fatalf("expected no items, got %#v", cmd.Items)
	}
}

func TestParseCommandSpecGetOptionalItems(t *testing.T) {
	cmd, err := parseCommandSpec("get-optional-items")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cmd.Action != "get-optional-items" {
		t.Fatalf("expected action get-optional-items, got %s", cmd.Action)
	}
	if len(cmd.Items) != 0 {
		t.Fatalf("expected no items, got %#v", cmd.Items)
	}
}

func TestParseCommandSpecInvalid(t *testing.T) {
	_, err := parseCommandSpec("install")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseCommandSpecLegacyActionInvalid(t *testing.T) {
	_, err := parseCommandSpec("uninstall:foo")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateCommandRunWithItems(t *testing.T) {
	err := validateCommand(Command{
		Action: "run",
		Items:  []string{"foo"},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestServiceInstallArgs(t *testing.T) {
	configPath := `C:\ProgramData\gorilla\config.yaml`
	got := serviceInstallArgs(configPath)
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
