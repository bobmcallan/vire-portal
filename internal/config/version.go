package config

import (
	"fmt"
)

// Version information (set via -ldflags during build).
var (
	Version   = "dev"
	Build     = "unknown"
	GitCommit = "unknown"
)

// GetVersion returns the current version string.
func GetVersion() string {
	return Version
}

// GetBuild returns the build timestamp.
func GetBuild() string {
	return Build
}

// GetGitCommit returns the git commit hash.
func GetGitCommit() string {
	return GitCommit
}

// GetFullVersion returns version with build info.
func GetFullVersion() string {
	return fmt.Sprintf("%s (build: %s, commit: %s)", Version, Build, GitCommit)
}
