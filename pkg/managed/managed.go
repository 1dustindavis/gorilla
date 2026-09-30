package managed

import (
	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/installer"
	"github.com/1dustindavis/gorilla/pkg/manifest"
)

// PreparedContext is the effective repository state prepared for one managed
// execution. It is a neutral data contract shared by cmd/gorilla and callers
// such as pkg/service; it intentionally contains no execution or UI policy.
type PreparedContext struct {
	Config    config.Configuration
	Manifests []manifest.Item
	Catalogs  map[int]map[string]catalog.Item
}

// RunResult is returned by a full managed convergence. Prepared reflects the
// exact context for that invocation; callers must not infer that it represents
// persistent configuration if the supplied run configuration itself was
// operation-scoped or transient. Build/import modes do not prepare repository
// execution state and therefore return the zero value.
type RunResult struct {
	Prepared PreparedContext
}

// ItemRunResult combines targeted execution evidence with the exact prepared
// repository state used for that targeted execution.
type ItemRunResult struct {
	// ExecutionPrepared is the exact context used by the targeted execution.
	// It may contain operation-scoped transient state, such as the temporary
	// one-time-removal manifest used by RemoveItem, and therefore must not be
	// treated automatically as persistent post-operation snapshot input.
	ExecutionPrepared PreparedContext
	Execution         installer.Result
}

// RunFunc executes a full managed run and returns the prepared repository state
// used for successful managed convergence.
type RunFunc func(config.Configuration) (RunResult, error)

// ItemRunFunc executes one accepted targeted managed item action.
type ItemRunFunc func(config.Configuration, string, string) (ItemRunResult, error)
