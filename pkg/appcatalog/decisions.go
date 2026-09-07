package appcatalog

// DecideActions returns service-owned decisions in documented precedence order.
// Selection is independent of presence: an installed, unselected app can be
// adopted, and an absent selected app can have its selection withdrawn.
// Callers must resolve fresh policy/capabilities and recheck before mutation.
func DecideActions(observed ObservedState, policy Policy, capabilities Capabilities, busy bool) Actions {
	deny := func(reason string) ActionDecision { return ActionDecision{Reason: reason} }
	denyBoth := func(reason string) Actions { return Actions{deny(reason), deny(reason)} }
	if !policy.Optional {
		return denyBoth("not_optional")
	}
	if policy.RequiredUninstall && (policy.RequiredInstall || policy.RequiredDependency) {
		return denyBoth("policy_conflict")
	}
	if policy.RequiredUninstall {
		return denyBoth("managed_uninstall")
	}
	if policy.RequiredInstall {
		return denyBoth("required_install")
	}
	if busy {
		return denyBoth("operation_active")
	}
	if policy.Selection != NoSelection && policy.Selection != KeepInstalled {
		return denyBoth("invalid_selection")
	}
	switch observed {
	case Unknown:
		return denyBoth("state_unknown")
	case DetectionFailed:
		return denyBoth("detection_failed")
	case Absent, Installed, UpdateAvailable:
		// Known presence; installed version may still be unavailable.
	default:
		return denyBoth("state_unknown")
	}

	allowed := ActionDecision{Allowed: true}
	actions := Actions{Install: allowed, Remove: allowed}
	if !capabilities.CanInstall {
		actions.Install = deny("install_unavailable")
	} else if policy.Selection == KeepInstalled && observed != Absent {
		// Selected apps update automatically; no separate Update button in v2.
		actions.Install = deny("already_selected")
	}
	if policy.RequiredDependency {
		actions.Remove = deny("required_dependency")
	} else if observed == Absent {
		if policy.Selection != KeepInstalled {
			actions.Remove = deny("already_absent")
		}
		// Withdrawing selection needs no uninstaller for a verified absent app.
	} else if !capabilities.CanRemove {
		actions.Remove = deny("remove_unavailable")
	}
	return actions
}

type Execution string

const (
	ExecutionCompleted   Execution = "Completed"
	ExecutionSkipped     Execution = "Skipped"
	ExecutionFailed      Execution = "Failed"
	ExecutionInterrupted Execution = "Interrupted"
)

type Verification string

const (
	Satisfied    Verification = "Satisfied"
	NotSatisfied Verification = "NotSatisfied"
	NotVerified  Verification = "NotVerified"
)

// DecideResult never promotes execution errors to success based on observation.
// Verification covers the requested target state AND persisted selection; it
// must come from fresh evidence, not the legacy actionNeeded boolean alone.
func DecideResult(execution Execution, verification Verification) Result {
	switch execution {
	case ExecutionFailed:
		return Result{Failed, "execution_failed"}
	case ExecutionInterrupted:
		return Result{Interrupted, "execution_interrupted"}
	case ExecutionCompleted, ExecutionSkipped:
		// Only completed/skipped execution can establish success.
	default:
		return Result{Unverified, "execution_unknown"}
	}
	switch verification {
	case NotSatisfied:
		return Result{Failed, "postcondition_failed"}
	case Satisfied:
		if execution == ExecutionSkipped {
			return Result{AlreadySatisfied, ""}
		}
		return Result{Succeeded, ""}
	default:
		return Result{Unverified, "verification_unavailable"}
	}
}
