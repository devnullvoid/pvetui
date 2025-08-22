package version

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestGetBuildInfo(t *testing.T) {
	info := GetBuildInfo()

	if info == nil {
		t.Fatal("GetBuildInfo() returned nil")
	}

	// Check that required fields are present
	if info.Version == "" {
		t.Error("Version should not be empty")
	}

	if info.GoVersion == "" {
		t.Error("GoVersion should not be empty")
	}

	if info.OS == "" {
		t.Error("OS should not be empty")
	}

	if info.Arch == "" {
		t.Error("Arch should not be empty")
	}
}

func TestGetVersionString(t *testing.T) {
	versionStr := GetVersionString()

	if versionStr == "" {
		t.Error("GetVersionString() should not return empty string")
	}

	if versionStr[0] != 'v' {
		t.Error("GetVersionString() should start with 'v'")
	}
}

func TestGetFullVersionString(t *testing.T) {
	fullVersion := GetFullVersionString()

	if fullVersion == "" {
		t.Error("GetFullVersionString() should not return empty string")
	}

	if fullVersion[0] != 'v' {
		t.Error("GetFullVersionString() should start with 'v'")
	}
}

func TestGetBuildDate(t *testing.T) {
	info := GetBuildInfo()

	if info.BuildDate == "unknown" {
		// If build date is unknown, the function should return an error
		_, err := GetBuildDate()
		if err == nil {
			t.Error("GetBuildDate() should return error when build date is unknown")
		}
		return
	}

	// If build date is set, it should parse correctly
	buildTime, err := GetBuildDate()
	if err != nil {
		t.Errorf("GetBuildDate() failed to parse build date: %v", err)
	}

	// Build time should be reasonable (not zero time)
	if buildTime.IsZero() {
		t.Error("GetBuildDate() returned zero time")
	}

	// Build time should be in the past
	if buildTime.After(time.Now()) {
		t.Error("GetBuildDate() returned future time")
	}
}

func TestIsDevBuild(t *testing.T) {
	isDev := IsDevBuild()

	// This test is mostly to ensure the function doesn't panic
	// The actual value depends on how the binary was built
	_ = isDev
}

func TestGitHubUserConstant(t *testing.T) {
	if GitHubUser == "" {
		t.Error("GitHubUser constant should not be empty")
	}

	if GitHubUser != "devnullvoid" {
		t.Errorf("GitHubUser constant should be 'devnullvoid', got: %s", GitHubUser)
	}
}

func TestProjectNameConstant(t *testing.T) {
	if ProjectName == "" {
		t.Error("ProjectName constant should not be empty")
	}

	if ProjectName != "peevetui" {
		t.Errorf("ProjectName constant should be 'peevetui', got: %s", ProjectName)
	}
}

func TestGetGitHubURL(t *testing.T) {
	url := GetGitHubURL()
	expectedURL := fmt.Sprintf("https://github.com/%s/%s", GitHubUser, ProjectName)

	if url == "" {
		t.Error("GetGitHubURL() should not return empty string")
	}

	if url != expectedURL {
		t.Errorf("GetGitHubURL() returned unexpected URL: %s, expected: %s", url, expectedURL)
	}
}

func TestGetGitHubReleaseURL(t *testing.T) {
	url := GetGitHubReleaseURL()
	expectedURL := fmt.Sprintf("https://github.com/%s/%s/releases", GitHubUser, ProjectName)

	if url == "" {
		t.Error("GetGitHubReleaseURL() should not return empty string")
	}

	if url != expectedURL {
		t.Errorf("GetGitHubReleaseURL() returned unexpected URL: %s, expected: %s", url, expectedURL)
	}
}

func TestAuthorConstant(t *testing.T) {
	if Author == "" {
		t.Error("Author constant should not be empty")
	}

	if Author != "Jon Rogers" {
		t.Errorf("Author constant should be 'Jon Rogers', got: %s", Author)
	}
}

func TestLicenseConstant(t *testing.T) {
	if License == "" {
		t.Error("License constant should not be empty")
	}

	if License != "MIT License" {
		t.Errorf("License constant should be 'MIT License', got: %s", License)
	}
}

func TestGetCopyrightYearRange(t *testing.T) {
	yearRange := GetCopyrightYearRange()
	currentYear := time.Now().Year()

	// Should contain the current year
	if !strings.Contains(yearRange, fmt.Sprintf("%d", currentYear)) {
		t.Errorf("GetCopyrightYearRange() should contain current year %d, got: %s", currentYear, yearRange)
	}

	// Should be in expected format (either "2025" or "2025-2026")
	if !strings.Contains(yearRange, "2025") {
		t.Errorf("GetCopyrightYearRange() should contain start year 2025, got: %s", yearRange)
	}

	// Should not be empty
	if yearRange == "" {
		t.Error("GetCopyrightYearRange() should not return empty string")
	}
}
