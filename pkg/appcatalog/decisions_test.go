package appcatalog

import (
	"bufio"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestPlannedProtocolExamples(t *testing.T) {
	f, err := os.Open("../../gorilla-ui/docs/app-catalog-v2-examples.ndjson")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var listed, queued, terminal bool
	for scanner.Scan() {
		var envelope struct {
			Version     string          `json:"version"`
			MessageType string          `json:"messageType"`
			Operation   string          `json:"operation"`
			OperationID string          `json:"operationId"`
			Payload     json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Version != "v2" {
			t.Fatalf("planned contract used version %q", envelope.Version)
		}
		if envelope.MessageType == "Response" && envelope.Operation == "ListOptionalInstalls" {
			var payload struct {
				Items []Item `json:"items"`
			}
			if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(payload.Items, loadExamples(t).Items) {
				t.Fatal("NDJSON list drifted from shared contract examples")
			}
			listed = true
		}
		if envelope.MessageType == "Event" {
			var operation Operation
			if err := json.Unmarshal(envelope.Payload, &operation); err != nil {
				t.Fatal(err)
			}
			if operation.OperationID != envelope.OperationID || operation.ItemName != "Pending" || operation.Action != InstallAction {
				t.Fatal("operation lost requested item/action identity")
			}
			switch operation.Phase {
			case Queued:
				queued = true
				if operation.Result != nil || operation.ProgressPercent != nil {
					t.Fatal("queued operation claimed a result or measured progress")
				}
			case Completed:
				terminal = true
				if operation.Result == nil || *operation.Result != DecideResult(ExecutionCompleted, NotVerified) {
					t.Fatal("unverified execution claimed success")
				}
			default:
				t.Fatalf("unexpected example phase %q", operation.Phase)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if !listed || !queued || !terminal {
		t.Fatal("missing list or lifecycle examples")
	}
}

type contractExamples struct {
	PolicyCases []struct {
		Name         string           `json:"name"`
		State        ObservedState    `json:"state"`
		Requirement  RequirementState `json:"requirement"`
		Policy       Policy           `json:"policy"`
		Capabilities Capabilities     `json:"capabilities"`
		Busy         bool             `json:"busy"`
		Expected     Actions          `json:"expected"`
	} `json:"policyCases"`
	ResultCases []struct {
		Name         string       `json:"name"`
		Execution    Execution    `json:"execution"`
		Verification Verification `json:"verification"`
		Expected     Result       `json:"expected"`
	} `json:"resultCases"`
	Items []Item `json:"items"`
}

func loadExamples(t *testing.T) contractExamples {
	t.Helper()
	f, err := os.Open("testdata/contract.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var examples contractExamples
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&examples); err != nil {
		t.Fatal(err)
	}
	return examples
}

func TestContractPolicyExamples(t *testing.T) {
	for _, example := range loadExamples(t).PolicyCases {
		t.Run(example.Name, func(t *testing.T) {
			got := DecideActionsWithRequirement(example.State, example.Requirement, example.Policy, example.Capabilities, example.Busy)
			if got != example.Expected {
				t.Fatalf("got %+v, want %+v", got, example.Expected)
			}
		})
	}
}

func TestScriptRequirementCanAuthorizeActionsWithoutClaimingPresence(t *testing.T) {
	policy := Policy{Optional: true, Selection: NoSelection}
	caps := Capabilities{CanInstall: true, CanRemove: true}

	needed := DecideActionsWithRequirement(Unknown, RequirementNotSatisfied, policy, caps, false)
	if !needed.Install.Allowed || needed.Remove.Allowed {
		t.Fatalf("not-satisfied script result produced wrong actions: %+v", needed)
	}

	satisfied := DecideActionsWithRequirement(Unknown, RequirementSatisfied, policy, caps, false)
	if !satisfied.Install.Allowed || !satisfied.Remove.Allowed {
		t.Fatalf("satisfied script result produced wrong actions: %+v", satisfied)
	}

	failed := DecideActionsWithRequirement(DetectionFailed, RequirementNotSatisfied, policy, caps, false)
	if failed.Install.Allowed || failed.Remove.Allowed || failed.Install.Reason != "detection_failed" {
		t.Fatalf("detection failure did not fail closed: %+v", failed)
	}
}

func TestContractResultExamples(t *testing.T) {
	for _, example := range loadExamples(t).ResultCases {
		t.Run(example.Name, func(t *testing.T) {
			if got := DecideResult(example.Execution, example.Verification); got != example.Expected {
				t.Fatalf("got %+v, want %+v", got, example.Expected)
			}
		})
	}
}

func TestPolicySafetyAcrossStatesAndSelections(t *testing.T) {
	for _, state := range []ObservedState{Absent, Installed, UpdateAvailable, Unknown, DetectionFailed, "future"} {
		for _, selection := range []Selection{NoSelection, KeepInstalled, "Remove", ""} {
			for _, caps := range []Capabilities{{}, {CanInstall: true}, {CanRemove: true}, {true, true}} {
				policies := []Policy{
					{Selection: selection}, // Not optional, regardless of state.
					{Optional: true, RequiredInstall: true, Selection: selection},
					{Optional: true, RequiredUninstall: true, Selection: selection},
					{Optional: true, RequiredInstall: true, RequiredUninstall: true, Selection: selection},
				}
				for _, policy := range policies {
					actions := DecideActions(state, policy, caps, false)
					if actions.Install.Allowed || actions.Remove.Allowed {
						t.Fatalf("policy bypass: state=%s policy=%+v caps=%+v", state, policy, caps)
					}
				}
				policy := Policy{Optional: true, Selection: selection}
				if actions := DecideActions(state, policy, caps, true); actions.Install.Allowed || actions.Remove.Allowed {
					t.Fatal("active operation allowed another mutation")
				}
				policy.RequiredDependency = true
				if DecideActions(state, policy, caps, false).Remove.Allowed {
					t.Fatal("allowed removal of a needed dependency")
				}
			}
		}
	}
}

func TestExecutionFailureCannotBecomeSuccess(t *testing.T) {
	for _, verification := range []Verification{Satisfied, NotSatisfied, NotVerified, ""} {
		if got := DecideResult(ExecutionFailed, verification); got.Outcome != Failed {
			t.Fatalf("execution failure hidden by %q: %+v", verification, got)
		}
		if got := DecideResult(ExecutionInterrupted, verification); got.Outcome != Interrupted {
			t.Fatalf("interruption hidden by %q: %+v", verification, got)
		}
	}
}

func TestItemExamplesRoundTripWithoutLosingUnknowns(t *testing.T) {
	examples := loadExamples(t)
	if len(examples.Items) == 0 {
		t.Fatal("missing item examples")
	}
	for _, item := range examples.Items {
		data, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		var copy Item
		if err := json.Unmarshal(data, &copy); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(item, copy) {
			t.Fatalf("item did not round trip: %s", data)
		}
	}
}
