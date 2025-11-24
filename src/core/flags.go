package core

import (
	"time"
)

// Flags represents command-line flags and configuration
type Flags struct {
	// Existing fields
	DryRun          bool
	Update          bool
	File            string
	Directory       string
	GitHubToken     string
	Stable          *uint
	Entries         []string // For tracking entries
	Days            *uint    // Days parameter
	ContinueOnError bool     // Continue on error flag

	// New cache fields
	Cache        *Cache
	CacheEnabled bool
	CacheTTL     time.Duration
}

// NewFlags creates a new Flags instance with default cache settings
func NewFlags() *Flags {
	return &Flags{
		CacheEnabled: true,           // Cache enabled by default
		CacheTTL:     24 * time.Hour, // 24 hour default TTL
		Entries:      []string{},     // Initialize empty slice
	}
}

// InitializeCache initializes the cache based on flags
func (f *Flags) InitializeCache() error {
	cache, err := NewCache(f.CacheTTL, f.CacheEnabled)
	if err != nil {
		return err
	}
	f.Cache = cache
	return nil
}
