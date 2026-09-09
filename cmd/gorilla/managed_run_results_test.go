package main

import (
	"reflect"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/installer"
	"github.com/1dustindavis/gorilla/pkg/process"
)

func TestLogManagedResultFailuresSurfacesStructuredFailures(t *testing.T) {
	previous := managedResultWarnFunc
	t.Cleanup(func() { managedResultWarnFunc = previous })

	var warnings [][]interface{}
	managedResultWarnFunc = func(fields ...interface{}) {
		warnings = append(warnings, append([]interface{}(nil), fields...))
	}

	results := []process.ItemResult{
		{
			ItemName: "Runtime",
			Result: installer.Result{
				ItemName: "Runtime",
				Action:   "install",
				Outcome:  installer.OutcomeSucceeded,
			},
		},
		{
			ItemName: "Parent",
			Result: installer.Result{
				ItemName:  "Parent",
				Action:    "install",
				Outcome:   installer.OutcomeFailed,
				ErrorCode: "dependency_failed",
				Message:   "Dependency Runtime did not complete",
			},
		},
	}

	logManagedResultFailures("install", results)

	want := [][]interface{}{
		{"Managed", "install", "failed:", "Parent", "code=", "dependency_failed", "message=", "Dependency Runtime did not complete"},
	}
	if !reflect.DeepEqual(warnings, want) {
		t.Fatalf("unexpected warnings: got %#v, want %#v", warnings, want)
	}
}

func TestLogManagedResultFailuresIncludesFailuresWithoutOptionalDetails(t *testing.T) {
	previous := managedResultWarnFunc
	t.Cleanup(func() { managedResultWarnFunc = previous })

	var warnings [][]interface{}
	managedResultWarnFunc = func(fields ...interface{}) {
		warnings = append(warnings, append([]interface{}(nil), fields...))
	}

	logManagedResultFailures("uninstall", []process.ItemResult{
		{
			ItemName: "Broken",
			Result: installer.Result{
				Outcome: installer.OutcomeFailed,
			},
		},
	})

	want := [][]interface{}{{"Managed", "uninstall", "failed:", "Broken"}}
	if !reflect.DeepEqual(warnings, want) {
		t.Fatalf("unexpected warnings: got %#v, want %#v", warnings, want)
	}
}

func TestLogManagedResultFailuresIgnoresNonFailures(t *testing.T) {
	previous := managedResultWarnFunc
	t.Cleanup(func() { managedResultWarnFunc = previous })

	called := false
	managedResultWarnFunc = func(...interface{}) { called = true }

	logManagedResultFailures("update", []process.ItemResult{
		{ItemName: "Current", Result: installer.Result{Outcome: installer.OutcomeAlreadyCurrent}},
		{ItemName: "Updated", Result: installer.Result{Outcome: installer.OutcomeSucceeded}},
	})

	if called {
		t.Fatal("non-failing managed results emitted warnings")
	}
}
