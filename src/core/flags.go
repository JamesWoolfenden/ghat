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
	Deep            bool
	Sources         []string
	Exclude         string // regex pattern; matching scanned paths are skipped

	// New cache fields
	Cache        *Cache
	CacheEnabled bool
	CacheTTL     time.Duration

	Silent        bool // suppress diff output (used by org bulk mode)
	PinOnly       bool // pin current tag to SHA without checking for upgrades
	Substitutions []Substitution
	InputUpgrades []InputUpgrade

	OpenPR      bool
	AutoMerge   bool
	Branch      string
	PRToken     string
	HTTPTimeout time.Duration
}

// NewFlags creates a new Flags instance with default cache settings
func NewFlags() *Flags {
	return &Flags{
		CacheEnabled: true,
		CacheTTL:     24 * time.Hour,
		HTTPTimeout:  30 * time.Second,
		Entries:      []string{},
	}
}

// InitializeCache initializes the cache and applies startup configuration.
func (f *Flags) InitializeCache() error {
	if f.HTTPTimeout > 0 {
		SetHTTPTimeout(f.HTTPTimeout)
	}
	cache, err := NewCache(f.CacheTTL, f.CacheEnabled)
	if err != nil {
		return err
	}
	f.Cache = cache
	cfg := LoadConfig(f.Directory)
	f.Substitutions = cfg.Substitutions
	f.InputUpgrades = cfg.InputUpgrades
	return nil
}
