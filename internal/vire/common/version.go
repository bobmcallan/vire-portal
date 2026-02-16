package common

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// LoadVersionFromFile attempts to load version info from a .version file
// in the same directory as the binary. The file uses INI-style sections
// ([vire-portal], [vire-mcp]) — the section matching the binary name is used.
// Falls back to reading unsectioned keys for backwards compatibility.
// Values loaded from file are only used as fallbacks when ldflags weren't
// provided (i.e. still at defaults).
func LoadVersionFromFile() {
	exe, err := os.Executable()
	if err != nil {
		return
	}

	binaryName := filepath.Base(exe)
	versionFile := filepath.Join(filepath.Dir(exe), ".version")
	loadVersionFile(versionFile, binaryName)
}

// loadVersionFile parses a .version file with INI-style sections.
// It reads keys from the section matching targetSection (e.g. "vire-mcp").
// If no sections are present, keys are read globally (legacy format).
func loadVersionFile(path, targetSection string) {
	f, err := os.Open(path)
	if err != nil {
		return
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

		// Section header: [vire-portal] or [vire-mcp]
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			hasSections = true
			section := line[1 : len(line)-1]
			inSection = section == targetSection
			continue
		}

		// Skip keys outside our target section (when sections exist).
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
}
