package service

import (
	"errors"
	"testing"

	"github.com/1dustindavis/gorilla/pkg/appcatalog"
	"github.com/1dustindavis/gorilla/pkg/installer"
	"github.com/1dustindavis/gorilla/pkg/manifest"
)

func TestClassifyInstallOperationResult(t *testing.T) {
	satisfied := appcatalog.Item{Observation: appcatalog.Observation{State: appcatalog.Installed, InstallRequirement: appcatalog.RequirementSatisfied}}
	notSatisfied := appcatalog.Item{Observation: appcatalog.Observation{State: appcatalog.UpdateAvailable, InstallRequirement: appcatalog.RequirementNotSatisfied, DetailCode: "version_requirement_unsatisfied"}}
	unknown := appcatalog.Item{Observation: appcatalog.Observation{State: appcatalog.Unknown, InstallRequirement: appcatalog.RequirementUnknown, DetailCode: "ambiguous_registry_match"}}
	selected := manifest.Item{Installs: []string{"App"}}

	tests := []struct {
		name       string
		execution  installer.Result
		item       appcatalog.Item
		selection  manifest.Item
		selectErr  error
		want       appcatalog.Outcome
		wantCode   string
		wantDetail string
	}{
		{
			name:      "executed and verified",
			execution: installer.Result{Outcome: installer.OutcomeSucceeded},
			item:      satisfied,
			selection: selected,
			want:      appcatalog.Succeeded,
		},
		{
			name:      "already current and verified",
			execution: installer.Result{Outcome: installer.OutcomeAlreadyCurrent},
			item:      satisfied,
			selection: selected,
			want:      appcatalog.AlreadySatisfied,
		},
		{
			name:      "no execution needed and verified",
			item:      satisfied,
			selection: selected,
			want:      appcatalog.AlreadySatisfied,
		},
		{
			name:       "execution failure wins over observation",
			execution:  installer.Result{Outcome: installer.OutcomeFailed, ErrorCode: "download_failed", Message: "hash mismatch"},
			item:       satisfied,
			selection:  selected,
			want:       appcatalog.Failed,
			wantCode:   "execution_failed",
			wantDetail: "download_failed",
		},
		{
			name:       "postcondition remains unsatisfied",
			execution:  installer.Result{Outcome: installer.OutcomeSucceeded},
			item:       notSatisfied,
			selection:  selected,
			want:       appcatalog.Failed,
			wantCode:   "postcondition_failed",
			wantDetail: "version_requirement_unsatisfied",
		},
		{
			name:       "verification unavailable",
			execution:  installer.Result{Outcome: installer.OutcomeSucceeded},
			item:       unknown,
			selection:  selected,
			want:       appcatalog.Unverified,
			wantCode:   "verification_unavailable",
			wantDetail: "ambiguous_registry_match",
		},
		{
			name:       "selection was not persisted",
			execution:  installer.Result{Outcome: installer.OutcomeSucceeded},
			item:       satisfied,
			selection:  manifest.Item{},
			want:       appcatalog.Failed,
			wantCode:   "postcondition_failed",
			wantDetail: "install_selection_not_persisted",
		},
		{
			name:       "selection cannot be read",
			execution:  installer.Result{Outcome: installer.OutcomeSucceeded},
			item:       satisfied,
			selection:  selected,
			selectErr:  errors.New("read failed"),
			want:       appcatalog.Unverified,
			wantCode:   "verification_unavailable",
			wantDetail: "selection_read_failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyOperationResult(actionInstallItem, "App", tt.execution, tt.item, true, tt.selection, tt.selectErr)
			if got.Outcome != tt.want || got.Code != tt.wantCode || got.DetailCode != tt.wantDetail {
				t.Fatalf("got outcome=%q code=%q detail=%q, want outcome=%q code=%q detail=%q", got.Outcome, got.Code, got.DetailCode, tt.want, tt.wantCode, tt.wantDetail)
			}
		})
	}
}

func TestClassifyRemoveOperationResult(t *testing.T) {
	absent := appcatalog.Item{Observation: appcatalog.Observation{State: appcatalog.Absent}}
	unknown := appcatalog.Item{Observation: appcatalog.Observation{State: appcatalog.Unknown, DetailCode: "script_requirement_satisfied"}}
	installed := appcatalog.Item{Observation: appcatalog.Observation{State: appcatalog.Installed}}
	cleared := manifest.Item{}

	tests := []struct {
		name       string
		execution  installer.Result
		item       appcatalog.Item
		selection  manifest.Item
		want       appcatalog.Outcome
		wantCode   string
		wantDetail string
	}{
		{
			name:      "executed absent and forgotten",
			execution: installer.Result{Outcome: installer.OutcomeSucceeded},
			item:      absent,
			selection: cleared,
			want:      appcatalog.Succeeded,
		},
		{
			name:      "already absent and forgotten",
			item:      absent,
			selection: cleared,
			want:      appcatalog.AlreadySatisfied,
		},
		{
			name:       "absence cannot be established",
			execution:  installer.Result{Outcome: installer.OutcomeSucceeded},
			item:       unknown,
			selection:  cleared,
			want:       appcatalog.Unverified,
			wantCode:   "verification_unavailable",
			wantDetail: "script_requirement_satisfied",
		},
		{
			name:       "item remains installed",
			execution:  installer.Result{Outcome: installer.OutcomeSucceeded},
			item:       installed,
			selection:  cleared,
			want:       appcatalog.Failed,
			wantCode:   "postcondition_failed",
		},
		{
			name:       "install selection remains",
			execution:  installer.Result{Outcome: installer.OutcomeSucceeded},
			item:       absent,
			selection:  manifest.Item{Installs: []string{"App"}},
			want:       appcatalog.Failed,
			wantCode:   "postcondition_failed",
			wantDetail: "install_selection_not_cleared",
		},
		{
			name:       "persistent uninstall remains",
			execution:  installer.Result{Outcome: installer.OutcomeSucceeded},
			item:       absent,
			selection:  manifest.Item{Uninstalls: []string{"App"}},
			want:       appcatalog.Failed,
			wantCode:   "postcondition_failed",
			wantDetail: "persistent_uninstall_present",
		},
		{
			name:       "execution failure remains failure even if absent",
			execution:  installer.Result{Outcome: installer.OutcomeFailed, ErrorCode: "uninstaller_failed", Message: "exit 1"},
			item:       absent,
			selection:  cleared,
			want:       appcatalog.Failed,
			wantCode:   "execution_failed",
			wantDetail: "uninstaller_failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyOperationResult(actionRemoveItem, "App", tt.execution, tt.item, true, tt.selection, nil)
			if got.Outcome != tt.want || got.Code != tt.wantCode || got.DetailCode != tt.wantDetail {
				t.Fatalf("got outcome=%q code=%q detail=%q, want outcome=%q code=%q detail=%q", got.Outcome, got.Code, got.DetailCode, tt.want, tt.wantCode, tt.wantDetail)
			}
		})
	}
}

func TestClassifyOperationResultMissingCatalogItemIsUnverified(t *testing.T) {
	got := classifyOperationResult(actionInstallItem, "App", installer.Result{Outcome: installer.OutcomeSucceeded}, appcatalog.Item{}, false, manifest.Item{Installs: []string{"App"}}, nil)
	if got.Outcome != appcatalog.Unverified || got.Code != "verification_unavailable" || got.DetailCode != "catalog_item_unavailable" {
		t.Fatalf("unexpected missing-item result: %+v", got)
	}
}
