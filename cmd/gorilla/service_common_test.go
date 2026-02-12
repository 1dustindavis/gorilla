package main

import "testing"

func TestParseServiceCommandSpecInstall(t *testing.T) {
	cmd, err := parseServiceCommandSpec("install:GoogleChrome, 7zip")
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

func TestParseServiceCommandSpecRun(t *testing.T) {
	cmd, err := parseServiceCommandSpec("run")
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

func TestParseServiceCommandSpecInvalid(t *testing.T) {
	_, err := parseServiceCommandSpec("install")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateServiceCommandRunWithItems(t *testing.T) {
	err := validateServiceCommand(serviceCommand{
		Action: "run",
		Items:  []string{"foo"},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}
