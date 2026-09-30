// Package managed provides temporary aliases for the managedrun execution
// contract while PR A consumers are migrated to the more descriptive package.
// Deprecated: use github.com/1dustindavis/gorilla/pkg/managedrun.
package managed

import "github.com/1dustindavis/gorilla/pkg/managedrun"

type PreparedContext = managedrun.PreparedContext
type RunResult = managedrun.RunResult
type ItemRunResult = managedrun.ItemRunResult
type RunFunc = managedrun.RunFunc
type ItemRunFunc = managedrun.ItemRunFunc
