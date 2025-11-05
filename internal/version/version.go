package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

// BuildInfo contains build-time information
type BuildInfo struct {
	Version   string
	BuildDate string
	Commit    string
	GoVersion string
	OS        string
	Arch      string
}

// Global build info variables that will be set at build time via -ldflags
var (
	version   = "dev"
	buildDate = "unknown"
	commit    = "unknown"
)

// GetBuildInfo returns the current build information
// It first checks ldflags-injected values, then falls back to debug.ReadBuildInfo()
// for version information when installed via `go install`
func GetBuildInfo() *BuildInfo {
	info := &BuildInfo{
		Version:   version,
		BuildDate: buildDate,
		Commit:    commit,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}

	// If version is still "dev", try to get it from build info (for go install)
	if info.Version == "dev" {
		if buildInfo, ok := debug.ReadBuildInfo(); ok {
			// Try to extract version from module info
			if buildInfo.Main.Version != "" && buildInfo.Main.Version != "(devel)" {
				info.Version = strings.TrimPrefix(buildInfo.Main.Version, "v")
			}

			// Extract VCS info if available
			for _, setting := range buildInfo.Settings {
				switch setting.Key {
				case "vcs.revision":
					if info.Commit == "unknown" && len(setting.Value) >= 7 {
						info.Commit = setting.Value[:7]
					}
				case "vcs.time":
					if info.BuildDate == "unknown" {
						info.BuildDate = setting.Value
					}
				}
			}
		}
	}

	return info
}

// GetVersionString returns a formatted version string
func GetVersionString() string {
	info := GetBuildInfo()
	return fmt.Sprintf("v%s", info.Version)
}

// GetFullVersionString returns a detailed version string
func GetFullVersionString() string {
	info := GetBuildInfo()
	return fmt.Sprintf("v%s (%s)", info.Version, info.Commit)
}

// GetBuildDate returns the build date as a time.Time
func GetBuildDate() (time.Time, error) {
	info := GetBuildInfo()
	if info.BuildDate == "unknown" {
		return time.Time{}, fmt.Errorf("build date not available")
	}
	return time.Parse(time.RFC3339, info.BuildDate)
}

// IsDevBuild returns true if this is a development build
func IsDevBuild() bool {
	return version == "dev"
}

// GetGitHubURL returns the GitHub repository URL
func GetGitHubURL() string {
	return fmt.Sprintf("https://github.com/%s/%s", GitHubUser, ProjectName)
}

// GetGitHubReleaseURL returns the GitHub releases URL
func GetGitHubReleaseURL() string {
	return fmt.Sprintf("https://github.com/%s/%s/releases", GitHubUser, ProjectName)
}

// Project constants
const (
	Author      = "Jon Rogers"
	License     = "MIT License"
	GitHubUser  = "devnullvoid"
	ProjectName = "pvetui"
)

// GetCopyrightYearRange returns a copyright year range (e.g., "2025-2026")
func GetCopyrightYearRange() string {
	startYear := 2025 // Project start year
	currentYear := time.Now().Year()

	if currentYear == startYear {
		return fmt.Sprintf("%d", startYear)
	}

	return fmt.Sprintf("%d-%d", startYear, currentYear)
}
