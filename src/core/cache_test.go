package core

import (
	"fmt"
	"testing"
	"time"
)

func TestCache_SetAndGet(t *testing.T) {
	cache, err := NewCache(1*time.Hour, true)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Clear()

	url := "https://api.github.com/repos/test/repo"
	testData := map[string]interface{}{
		"name": "test-repo",
		"id":   12345,
	}

	// Set cache
	err = cache.Set(url, testData)
	if err != nil {
		t.Errorf("Failed to set cache: %v", err)
	}

	// Get cache
	cached, found := cache.Get(url)
	if !found {
		t.Error("Cache not found after setting")
	}

	cachedMap, ok := cached.(map[string]interface{})
	if !ok {
		t.Error("Cached data is not a map")
	}

	if cachedMap["name"] != "test-repo" {
		t.Errorf("Cached name = %v, want test-repo", cachedMap["name"])
	}
}

func TestCache_Expiration(t *testing.T) {
	// Create cache with very short TTL
	cache, err := NewCache(100*time.Millisecond, true)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Clear()

	url := "https://api.github.com/repos/test/repo"
	testData := "test data"

	// Set cache
	cache.Set(url, testData)

	// Should be cached immediately
	if _, found := cache.Get(url); !found {
		t.Error("Cache should be found immediately after setting")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	if _, found := cache.Get(url); found {
		t.Error("Cache should be expired after TTL")
	}
}

func TestCache_Disabled(t *testing.T) {
	cache, err := NewCache(1*time.Hour, false)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	if cache.enabled {
		t.Error("Cache should be disabled")
	}

	url := "https://api.github.com/test"
	testData := "test"

	// Set should do nothing
	cache.Set(url, testData)

	// Get should return not found
	if _, found := cache.Get(url); found {
		t.Error("Disabled cache should not return any data")
	}
}

func TestCache_Clear(t *testing.T) {
	cache, err := NewCache(1*time.Hour, true)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Clear()

	// Add multiple entries
	for i := 0; i < 5; i++ {
		url := fmt.Sprintf("https://api.github.com/test/%d", i)
		cache.Set(url, fmt.Sprintf("data-%d", i))
	}

	// Verify entries exist
	count, _, err := cache.Stats()
	if err != nil {
		t.Errorf("Failed to get stats: %v", err)
	}
	if count != 5 {
		t.Errorf("Expected 5 cache entries, got %d", count)
	}

	// Clear cache
	err = cache.Clear()
	if err != nil {
		t.Errorf("Failed to clear cache: %v", err)
	}

	// Verify cache is empty
	count, _, err = cache.Stats()
	if err != nil {
		t.Errorf("Failed to get stats after clear: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 cache entries after clear, got %d", count)
	}
}

func TestCache_Stats(t *testing.T) {
	cache, err := NewCache(1*time.Hour, true)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Clear()

	// Add entries
	for i := 0; i < 3; i++ {
		url := fmt.Sprintf("https://api.github.com/test/%d", i)
		cache.Set(url, map[string]interface{}{"id": i})
	}

	count, size, err := cache.Stats()
	if err != nil {
		t.Errorf("Failed to get stats: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 entries, got %d", count)
	}

	if size == 0 {
		t.Error("Expected non-zero cache size")
	}

	t.Logf("Cache has %d entries totaling %d bytes", count, size)
}

func TestCache_ClearExpired(t *testing.T) {
	cache, err := NewCache(100*time.Millisecond, true)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Clear()

	// Add entries
	cache.Set("https://api.github.com/test/1", "data1")
	cache.Set("https://api.github.com/test/2", "data2")

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Add new entry (not expired)
	cache2, _ := NewCache(1*time.Hour, true)
	cache2.dir = cache.dir // Use same cache dir
	cache2.Set("https://api.github.com/test/3", "data3")

	// Clear expired
	err = cache.ClearExpired()
	if err != nil {
		t.Errorf("Failed to clear expired: %v", err)
	}

	// Should only have the non-expired entry
	count, _, _ := cache.Stats()
	if count != 1 {
		t.Errorf("Expected 1 entry after clearing expired, got %d", count)
	}
}
