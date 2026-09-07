package status

import "time"

// ObservedState describes device evidence independently from requested policy.
type ObservedState string

const (
	Absent          ObservedState = "Absent"
	Installed       ObservedState = "Installed"
	UpdateAvailable ObservedState = "UpdateAvailable"
	Unknown         ObservedState = "Unknown"
	DetectionFailed ObservedState = "DetectionFailed"
)

// Observation is the richer result behind CheckStatus. ActionNeeded preserves
// the existing CLI/managed-run decision for the requested install type.
type Observation struct {
	State            ObservedState
	InstalledVersion *string
	CheckedAtUTC     time.Time
	DetailCode       string
	ActionNeeded     bool
}

// ResetRegistryCache starts a fresh detection pass. Callers should invoke it
// once before evaluating a related set of items so registry enumeration is
// shared within that pass but never reused as a later snapshot.
func ResetRegistryCache() {
	RegistryItems = nil
}

func observation(state ObservedState, version string, detail string, actionNeeded bool) Observation {
	var installedVersion *string
	if version != "" {
		installedVersion = &version
	}
	return Observation{
		State:            state,
		InstalledVersion: installedVersion,
		CheckedAtUTC:     time.Now().UTC(),
		DetailCode:       detail,
		ActionNeeded:     actionNeeded,
	}
}
