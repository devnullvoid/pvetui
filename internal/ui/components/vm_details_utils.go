package components

import (
	"regexp"
	"strings"

	"github.com/devnullvoid/pvetui/pkg/api"
)

// getFriendlyFilesystemName returns a user-friendly name for a filesystem.
func getFriendlyFilesystemName(fs api.Filesystem) string {
	// Try to extract a meaningful name from the mount point
	if fs.Mountpoint != "" {
		// Remove leading slash and common prefixes
		name := strings.TrimPrefix(fs.Mountpoint, "/")
		name = strings.TrimPrefix(name, "mnt/")
		name = strings.TrimPrefix(name, "media/")

		// If we have a reasonable name, use it
		if name != "" && name != "mnt" && name != "media" {
			return name
		}
	}

	// Fall back to the filesystem type
	if fs.Type != "" {
		return fs.Type
	}

	// Last resort
	return "Unknown"
}

// sanitizeDescription cleans up VM description text for display.
func sanitizeDescription(desc string) string {
	// Remove common HTML-like tags and excessive whitespace
	desc = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(desc, "")
	desc = regexp.MustCompile(`\s+`).ReplaceAllString(desc, " ")
	desc = strings.TrimSpace(desc)

	// Limit length to avoid cluttering the display
	if len(desc) > 100 {
		desc = desc[:97] + "..."
	}

	return desc
}
