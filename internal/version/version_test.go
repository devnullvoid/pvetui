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

func TestGetGitHubURL(t *testing.T) {
	url := GetGitHubURL()

	if url == "" {
		t.Error("GetGitHubURL() should not return empty string")
	}

	if url != "https://github.com/devnullvoid/proxmox-tui" {
		t.Errorf("GetGitHubURL() returned unexpected URL: %s", url)
	}
}

func TestGetGitHubReleaseURL(t *testing.T) {
	url := GetGitHubReleaseURL()

	if url == "" {
		t.Error("GetGitHubReleaseURL() should not return empty string")
	}

	if url != "https://github.com/devnullvoid/proxmox-tui/releases" {
		t.Errorf("GetGitHubReleaseURL() returned unexpected URL: %s", url)
	}
}

func TestAuthorConstant(t *testing.T) {
	if Author == "" {
		t.Error("Author constant should not be empty")
	}

	if Author != "devnullvoid" {
		t.Errorf("Author constant should be 'devnullvoid', got: %s", Author)
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

func TestGetCopyrightYear(t *testing.T) {
	year := GetCopyrightYear()
	currentYear := time.Now().Year()

	if year != currentYear {
		t.Errorf("GetCopyrightYear() returned %d, expected %d", year, currentYear)
	}

	// Year should be reasonable (not in the past or too far in the future)
	if year < 2020 || year > 2030 {
		t.Errorf("GetCopyrightYear() returned unreasonable year: %d", year)
	}
}

func TestGetCopyrightYearRange(t *testing.T) {
	yearRange := GetCopyrightYearRange()
	currentYear := time.Now().Year()

	// Should contain the current year
	if !strings.Contains(yearRange, fmt.Sprintf("%d", currentYear)) {
		t.Errorf("GetCopyrightYearRange() should contain current year %d, got: %s", currentYear, yearRange)
	}

	// Should be in expected format (either "2024" or "2024-2025")
	if !strings.Contains(yearRange, "2024") {
		t.Errorf("GetCopyrightYearRange() should contain start year 2024, got: %s", yearRange)
	}

	// Should not be empty
	if yearRange == "" {
		t.Error("GetCopyrightYearRange() should not return empty string")
	}
}
