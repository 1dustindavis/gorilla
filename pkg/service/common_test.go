package service

import "testing"

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
