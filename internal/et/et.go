// Package et provides optional electrictown integration for pi-go.
package et

import "context"

// Delegator abstracts task delegation to electrictown workers.
type Delegator interface {
	// Delegate sends a task to electrictown for parallel execution.
	Delegate(ctx context.Context, task string, opts *Options) (*Result, error)

	// Available checks whether electrictown is installed and configured.
	Available() bool
}

// Options configures an et delegation request.
type Options struct {
	// ConfigPath overrides the default electrictown.yaml location.
	ConfigPath string

	// OutputDir specifies where et should write results.
	OutputDir string

	// Iterate enables the iterative build/fix loop (--iterate).
	Iterate bool

	// ExtraArgs are additional CLI arguments passed to et.
	ExtraArgs []string
}

// Result holds the output from an et delegation.
type Result struct {
	// Output is the combined text output from et workers.
	Output string

	// Files lists any files created or modified by et.
	Files []string

	// ExitCode is the et process exit code.
	ExitCode int
}
