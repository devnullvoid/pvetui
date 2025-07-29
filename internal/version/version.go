package version

import (
	"fmt"
	"runtime"
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

// Global build info variables that will be set at build time
var (
	version   = "dev"
	buildDate = "unknown"
	commit    = "unknown"
)

// GetBuildInfo returns the current build information
func GetBuildInfo() *BuildInfo {
	return &BuildInfo{
		Version:   version,
		BuildDate: buildDate,
		Commit:    commit,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
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
	return "https://github.com/devnullvoid/proxmox-tui"
}

// GetGitHubReleaseURL returns the GitHub releases URL
func GetGitHubReleaseURL() string {
	return "https://github.com/devnullvoid/proxmox-tui/releases"
}

// Project constants
const (
	Author  = "devnullvoid"
	License = "MIT License"
)

// GetCopyrightYear returns the current year for copyright
func GetCopyrightYear() int {
	return time.Now().Year()
}

// GetCopyrightYearRange returns a copyright year range (e.g., "2024-2025")
func GetCopyrightYearRange() string {
	startYear := 2024 // Project start year
	currentYear := time.Now().Year()

	if currentYear == startYear {
		return fmt.Sprintf("%d", startYear)
	}

	return fmt.Sprintf("%d-%d", startYear, currentYear)
}
