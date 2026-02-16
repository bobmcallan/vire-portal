package common

import (
	"fmt"
)

// SchemaVersion is bumped whenever model structs or computation logic changes
// invalidate cached derived data. On startup, a mismatch triggers automatic
// purge of derived data (Portfolio, MarketData, Signals, Reports) while
// preserving user data (Strategy, KV).
const SchemaVersion = "5"

// Version variables injected at build time via ldflags
var (
	Version   = "dev"
	Build     = "unknown"
	GitCommit = "unknown"
)

// GetVersion returns the semantic version string
func GetVersion() string {
	return Version
}

// GetBuild returns the build timestamp
func GetBuild() string {
	return Build
}

// GetGitCommit returns the short git commit hash
func GetGitCommit() string {
	return GitCommit
}

// GetFullVersion returns a formatted version string with all build info
func GetFullVersion() string {
	return fmt.Sprintf("%s (build: %s, commit: %s)", Version, Build, GitCommit)
}
