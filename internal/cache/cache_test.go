package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestFileCache_SetGet tests basic set and get operations for FileCache
func TestFileCache_SetGet(t *testing.T) {
	dir := t.TempDir()
	c, err := NewFileCache(dir, true)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	type data struct{ Name string }
	key := "test-key"
	expected := data{Name: "value"}

	if err := c.Set(key, expected, time.Minute); err != nil {
		t.Fatalf("set error: %v", err)
	}

	var got data
	found, err := c.Get(key, &got)
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	if !found {
		t.Fatalf("expected item not found")
	}
	if got != expected {
		t.Fatalf("expected %v got %v", expected, got)
	}

	// Ensure file exists for persistence
	if _, err := os.Stat(filepath.Join(dir, key+".json")); err != nil {
		t.Fatalf("expected cache file: %v", err)
	}
}

// TestFileCache_TTL verifies that expired items are not returned
func TestFileCache_TTL(t *testing.T) {
	c, err := NewFileCache(t.TempDir(), false)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	if err := c.Set("k", "v", time.Millisecond); err != nil {
		t.Fatalf("set: %v", err)
	}
	time.Sleep(5 * time.Millisecond)

	var s string
	found, err := c.Get("k", &s)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if found {
		t.Fatalf("expected item to be expired")
	}
}

// TestFileCache_Persistence ensures data is loaded from disk when persisted
func TestFileCache_Persistence(t *testing.T) {
	dir := t.TempDir()
	c1, err := NewFileCache(dir, true)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_ = c1.Set("p", "val", time.Minute)

	// Create new cache with same directory
	c2, err := NewFileCache(dir, true)
	if err != nil {
		t.Fatalf("create2: %v", err)
	}

	var v string
	found, err := c2.Get("p", &v)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !found || v != "val" {
		t.Fatalf("expected persisted value, got %v found %v", v, found)
	}
}
