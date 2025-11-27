package vnc

import (
	"io/fs"
	"strings"
	"testing"
)

// TestNoVNCFilesEmbedded verifies that noVNC files are properly embedded.
func TestNoVNCFilesEmbedded(t *testing.T) {
	// Test that the embedded filesystem contains expected files
	expectedFiles := []string{
		"vnc.html",
		"vnc_lite.html",
		"app/ui.js",
		"core/rfb.js",
		"lib/pako/lib/zlib/inflate.js",
	}

	novncFS, err := fs.Sub(novncFiles, "novnc")
	if err != nil {
		t.Fatalf("Failed to create noVNC filesystem: %v", err)
	}

	for _, file := range expectedFiles {
		t.Run("file_"+file, func(t *testing.T) {
			_, err := fs.Stat(novncFS, file)
			if err != nil {
				t.Errorf("Expected file %s not found in embedded filesystem: %v", file, err)
			}
		})
	}
}

// TestNoVNCMainPage verifies that the main noVNC page can be read.
func TestNoVNCMainPage(t *testing.T) {
	novncFS, err := fs.Sub(novncFiles, "novnc")
	if err != nil {
		t.Fatalf("Failed to create noVNC filesystem: %v", err)
	}

	content, err := fs.ReadFile(novncFS, "vnc.html")
	if err != nil {
		t.Fatalf("Failed to read vnc.html: %v", err)
	}

	if len(content) == 0 {
		t.Error("vnc.html appears to be empty")
	}

	// Check for expected noVNC content
	contentStr := string(content)
	if !strings.Contains(contentStr, "noVNC") {
		t.Error("vnc.html does not appear to contain noVNC content")
	}
}
