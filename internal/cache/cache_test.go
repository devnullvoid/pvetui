package cache

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestFileCache_SetGet tests basic set and get operations for FileCache.
func TestFileCache_SetGet(t *testing.T) {
	dir := t.TempDir()

	c, err := NewFileCache(dir, true)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	type data struct{ Name string }

	key := "test-key"
	expected := data{Name: "value"}

	if setErr := c.Set(key, expected, time.Minute); setErr != nil {
		t.Fatalf("set error: %v", setErr)
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

// TestFileCache_TTL verifies that expired items are not returned.
func TestFileCache_TTL(t *testing.T) {
	c, err := NewFileCache(t.TempDir(), false)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	if setErr := c.Set("k", "v", time.Second); setErr != nil {
		t.Fatalf("set: %v", setErr)
	}

	time.Sleep(2 * time.Second)

	var s string

	found, err := c.Get("k", &s)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if found {
		t.Fatalf("expected item to be expired")
	}
}

// TestFileCache_Persistence ensures data is loaded from disk when persisted.
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

// resetCacheState resets package-level cache state for isolated testing.
func resetCacheState() {
	globalCache = nil
	cacheLogger = nil
	globalCacheDir = ""
	once = sync.Once{}
	cacheLoggerOnce = sync.Once{}
	namespacedCaches = make(map[string]Cache)
	namespacedCacheMu = sync.RWMutex{}
}

// TestNamespacedCacheIsolation verifies that namespaced caches are not affected by clearing the global cache.
func TestNamespacedCacheIsolation(t *testing.T) {
	resetCacheState()
	defer resetCacheState()

	cacheDir := t.TempDir()

	if err := InitGlobalCache(cacheDir); err != nil {
		t.Fatalf("InitGlobalCache: %v", err)
	}

	global := GetGlobalCache()
	defer func() {
		_ = global.Close()
	}()

	nsCache := GetNamespacedCache("test-plugin")
	defer func() {
		_ = nsCache.Close()
	}()

	type payload struct {
		Value string
	}

	item := payload{Value: "persist"}

	if err := nsCache.Set("ns-key", item, time.Hour); err != nil {
		t.Fatalf("namespaced set: %v", err)
	}

	if err := global.Clear(); err != nil {
		t.Fatalf("global clear: %v", err)
	}

	var got payload
	found, err := nsCache.Get("ns-key", &got)
	if err != nil {
		t.Fatalf("namespaced get: %v", err)
	}

	if !found {
		t.Fatal("expected namespaced cache item to persist after global clear")
	}

	if got != item {
		t.Fatalf("expected %v, got %v", item, got)
	}
}
