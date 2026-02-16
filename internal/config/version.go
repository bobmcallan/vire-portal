package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// LoadVersionFromFile reads version info from a .version file in the same
// directory as the binary. The file uses INI-style sections ([vire-portal],
// [vire-mcp]) — the section matching the binary name is used.
// Falls back to reading unsectioned keys for backwards compatibility.
// Values are only used as fallbacks when ldflags weren't provided.
func LoadVersionFromFile() string {
	exePath, err := os.Executable()
	if err != nil {
		return Version
	}

	binaryName := filepath.Base(exePath)
	versionFile := filepath.Join(filepath.Dir(exePath), ".version")

	f, err := os.Open(versionFile)
	if err != nil {
		return Version
	}
	defer f.Close()

	inSection := false
	hasSections := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			hasSections = true
			section := line[1 : len(line)-1]
			inSection = section == binaryName
			continue
		}

		if hasSections && !inSection {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "version":
			if Version == "dev" {
				Version = val
			}
		case "build":
			if Build == "unknown" {
				Build = val
			}
		}
	}

	return Version
}
