// Package version holds build-time injected version information.
//
// Variables Version, BuildTime, and GitCommit are set
// at build time via -ldflags "-X". GoVersion is obtained
// at runtime via runtime.Version(). When built directly with
// 'go build' (without Makefile), they default to "dev"/"unknown".
package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the semantic version or git describe output.
	Version = "dev"
	// BuildTime is the UTC build timestamp in RFC3339 format.
	BuildTime = "unknown"
	// GitCommit is the short SHA of the git commit.
	GitCommit = "unknown"
)

// GoVersion returns the Go runtime version at execution time.
func GoVersion() string {
	return runtime.Version()
}

// Info returns a multi-line formatted version string.
func Info() string {
	return fmt.Sprintf(
		"Version:    %s\nBuilt:      %s\nGit Commit: %s\nGo Version: %s",
		Version, BuildTime, GitCommit, GoVersion(),
	)
}

// Short returns the version string only.
func Short() string {
	return Version
}
