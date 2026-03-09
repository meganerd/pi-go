// Package version provides build information set via ldflags.
package version

import "fmt"

// These variables are set by the linker via -ldflags.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// BuildInfo holds build metadata.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

// Info returns the current build information.
func Info() BuildInfo {
	return BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}
}

// String returns a formatted version string.
func (b BuildInfo) String() string {
	return fmt.Sprintf("pi-go %s (commit: %s, built: %s)", b.Version, b.Commit, b.Date)
}
