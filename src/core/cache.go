package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
)

// CacheEntry represents a cached API response
type CacheEntry struct {
	Data      interface{} `json:"data"`
	ExpiresAt time.Time   `json:"expires_at"`
	URL       string      `json:"url"`
}

// Cache handles caching of GitHub API responses
type Cache struct {
	dir     string
	ttl     time.Duration
	enabled bool
}

// NewCache creates a new cache instance
// ttl is the time-to-live for cached entries (e.g., 24 hours)
func NewCache(ttl time.Duration, enabled bool) (*Cache, error) {
	if !enabled {
		return &Cache{enabled: false}, nil
	}

	cacheDir := filepath.Join(os.TempDir(), "ghat-cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		log.Warn().Err(err).Msg("Failed to create cache directory, caching disabled")
		return &Cache{enabled: false}, nil
	}

	log.Debug().Str("dir", cacheDir).Dur("ttl", ttl).Msg("Cache initialized")

	return &Cache{
		dir:     cacheDir,
		ttl:     ttl,
		enabled: true,
	}, nil
}

// getCacheKey generates a cache key from a URL
func (c *Cache) getCacheKey(url string) string {
	hash := sha256.Sum256([]byte(url))
	return hex.EncodeToString(hash[:])
}

// Get retrieves a cached response
// Returns the cached data and true if found and not expired, otherwise nil and false
func (c *Cache) Get(url string) (interface{}, bool) {
	if !c.enabled {
		return nil, false
	}

	key := c.getCacheKey(url)
	cachePath := filepath.Join(c.dir, key)

	data, err := os.ReadFile(cachePath)
	if err != nil {
		log.Debug().Str("url", url).Msg("Cache miss")
		return nil, false
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		log.Debug().Err(err).Str("url", url).Msg("Failed to unmarshal cache entry")
		os.Remove(cachePath) // Remove corrupted cache file
		return nil, false
	}

	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		log.Debug().Str("url", url).Msg("Cache expired")
		os.Remove(cachePath)
		return nil, false
	}

	log.Debug().Str("url", url).Msg("Cache hit")
	return entry.Data, true
}

// Set stores a response in the cache
func (c *Cache) Set(url string, data interface{}) error {
	if !c.enabled {
		return nil
	}

	key := c.getCacheKey(url)
	cachePath := filepath.Join(c.dir, key)

	entry := CacheEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(c.ttl),
		URL:       url,
	}

	jsonData, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	if err := os.WriteFile(cachePath, jsonData, 0644); err != nil {
		log.Warn().Err(err).Str("url", url).Msg("Failed to write cache")
		return err
	}

	log.Debug().Str("url", url).Time("expires", entry.ExpiresAt).Msg("Cached response")
	return nil
}

// Clear removes all cached entries
func (c *Cache) Clear() error {
	if !c.enabled {
		return nil
	}

	if err := os.RemoveAll(c.dir); err != nil {
		return err
	}

	// Recreate the directory
	return os.MkdirAll(c.dir, 0755)
}

// ClearExpired removes expired cache entries
func (c *Cache) ClearExpired() error {
	if !c.enabled {
		return nil
	}

	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return err
	}

	now := time.Now()
	removed := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		cachePath := filepath.Join(c.dir, entry.Name())
		data, err := os.ReadFile(cachePath)
		if err != nil {
			continue
		}

		var cacheEntry CacheEntry
		if err := json.Unmarshal(data, &cacheEntry); err != nil {
			os.Remove(cachePath) // Remove corrupted file
			continue
		}

		if now.After(cacheEntry.ExpiresAt) {
			os.Remove(cachePath)
			removed++
		}
	}

	if removed > 0 {
		log.Info().Int("count", removed).Msg("Removed expired cache entries")
	}

	return nil
}

// Stats returns cache statistics
func (c *Cache) Stats() (int, int64, error) {
	if !c.enabled {
		return 0, 0, nil
	}

	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return 0, 0, err
	}

	var totalSize int64
	count := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		totalSize += info.Size()
		count++
	}

	return count, totalSize, nil
}

// IsEnabled returns whether the cache is enabled
func (c *Cache) IsEnabled() bool {
	return c.enabled
}
