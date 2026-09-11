// Package appcatalog defines App Catalog state, policy, action, and result contracts.
// These types are used by the live v1 service protocol; see
// gorilla-ui/docs/app-catalog-contract.md and app-catalog-recovery.md.
package appcatalog

import "time"

type ObservedState string

const (
	Absent          ObservedState = "Absent"
	Installed       ObservedState = "Installed"
	UpdateAvailable ObservedState = "UpdateAvailable"
	Unknown         ObservedState = "Unknown"
	DetectionFailed ObservedState = "DetectionFailed"
)

// RequirementState records whether the selected check says installation work
// is needed. It is separate from observed presence because a script can
// establish satisfaction without distinguishing absence from an older install.
type RequirementState string

const (
	RequirementUnknown      RequirementState = "Unknown"
	RequirementSatisfied    RequirementState = "Satisfied"
	RequirementNotSatisfied RequirementState = "NotSatisfied"
)

// Observation describes evidence, not policy or a pending operation. Installed
// establishes presence only; it does not imply a known version or target match.
type Observation struct {
	State              ObservedState    `json:"state"`
	InstalledVersion   *string          `json:"installedVersion"`
	CheckedAtUTC       *time.Time       `json:"checkedAtUtc"`
	DetailCode         string           `json:"detailCode"`
	InstallRequirement RequirementState `json:"installRequirement"`
}

type Selection string

const (
	NoSelection   Selection = "None"
	KeepInstalled Selection = "Install"
)

// Policy must be resolved from trusted manifests, excluding the service's
// user-selection manifest from RequiredInstall/RequiredUninstall. RequiredDependency
// includes dependencies still needed by required or other selected applications.
type Policy struct {
	Optional           bool      `json:"optional"`
	RequiredInstall    bool      `json:"requiredInstall"`
	RequiredUninstall  bool      `json:"requiredUninstall"`
	RequiredDependency bool      `json:"requiredDependency"`
	Selection          Selection `json:"selection"`
}

// Capabilities indicate validated catalog recipes, not an observation of presence.
type Capabilities struct {
	CanInstall bool `json:"canInstall"`
	CanRemove  bool `json:"canRemove"`
}

type ActionDecision struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason"`
}

type Actions struct {
	Install ActionDecision `json:"install"`
	Remove  ActionDecision `json:"remove"`
}

type OperationPhase string

const (
	Queued    OperationPhase = "Queued"
	Running   OperationPhase = "Running"
	Completed OperationPhase = "Completed"
)

type Action string

const (
	InstallAction Action = "Install"
	RemoveAction  Action = "Remove"
)

type Outcome string

const (
	Succeeded        Outcome = "Succeeded"
	AlreadySatisfied Outcome = "AlreadySatisfied"
	Failed           Outcome = "Failed"
	Unverified       Outcome = "Unverified"
	Interrupted      Outcome = "Interrupted"
)

type Result struct {
	Outcome Outcome `json:"outcome"`
	Code    string  `json:"code"`
}

// Operation separates execution phase from terminal result. Progress is null
// unless measured; a dropped client connection is not a terminal result.
type Operation struct {
	OperationID     string         `json:"operationId"`
	ItemName        string         `json:"itemName"`
	Action          Action         `json:"action"`
	Phase           OperationPhase `json:"phase"`
	ProgressPercent *int           `json:"progressPercent"`
	Result          *Result        `json:"result"`
}

// Item is the App Catalog list item contract. Nil fields serialize as null
// intentionally so unknown evidence is distinguishable from a supplied value.
// TargetVersion is catalog display metadata, not a detection or verification
// threshold. The selected pkg/status check owns version requirements.
type Item struct {
	ItemName        string      `json:"itemName"`
	DisplayName     string      `json:"displayName"`
	Description     string      `json:"description,omitempty"`
	Catalog         string      `json:"catalog"`
	TargetVersion   *string     `json:"targetVersion"`
	Observation     Observation `json:"observation"`
	Policy          Policy      `json:"policy"`
	Actions         Actions     `json:"actions"`
	ActiveOperation *Operation  `json:"activeOperation"`
	LastOperation   *Operation  `json:"lastOperation"`
}
