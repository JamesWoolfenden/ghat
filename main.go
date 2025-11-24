package main

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/jameswoolfenden/ghat/src/core"
	"github.com/jameswoolfenden/ghat/src/version"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
	"moul.io/banner"
)

func main() {
	fmt.Println(banner.Inline("ghat"))
	fmt.Println("version:", version.Version)

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	var myFlags core.Flags

	app := &cli.App{
		EnableBashCompletion: true,
		Copyright:            "James Woolfenden",
		Flags:                []cli.Flag{},
		Commands: []*cli.Command{
			{
				Name:      "version",
				Aliases:   []string{"v"},
				Usage:     "Outputs the application version",
				UsageText: "ghat version",
				Action: func(*cli.Context) error {
					fmt.Println(version.Version)

					return nil
				},
			},
			//{
			//	Name:      "swot",
			//	Aliases:   []string{"a"},
			//	Usage:     "updates GHA versions for hashes",
			//	UsageText: "ghat swot",
			//	Action: func(*cli.Context) error {
			//		return myFlags.Action("swot")
			//	},
			//	Flags: []cli.Flag{
			//		&cli.StringFlag{
			//			Name:        "file",
			//			Aliases:     []string{"f"},
			//			Usage:       "GHA file to parse",
			//			Destination: &myFlags.File,
			//			Category:    "files",
			//		},
			//		&cli.StringFlag{
			//			Name:        "directory",
			//			Aliases:     []string{"d"},
			//			Usage:       "Destination to update GHAs",
			//			Value:       ".",
			//			Destination: &myFlags.Directory,
			//			Category:    "files",
			//		},
			//		&cli.UintFlag{
			//			Name:        "stable",
			//			Aliases:     []string{"s"},
			//			Usage:       "days to wait for stabilisation of release",
			//			Value:       0,
			//			Destination: myFlags.Days,
			//			DefaultText: "0",
			//			Category:    "delay",
			//		},
			//		&cli.StringFlag{
			//			Name:        "token",
			//			Aliases:     []string{"t"},
			//			Usage:       "Github PAT token",
			//			Destination: &myFlags.GitHubToken,
			//			Category:    "authentication",
			//			EnvVars:     []string{"GITHUB_TOKEN", "GITHUB_API"},
			//		},
			//		&cli.BoolFlag{
			//			Name:        "dry-run",
			//			Usage:       "show but don't write changes",
			//			Destination: &myFlags.DryRun,
			//			Value:       false,
			//		},
			//		&cli.BoolFlag{
			//			Name:        "continue-on-error",
			//			Usage:       "just keep going",
			//			Destination: &myFlags.ContinueOnError,
			//			Value:       false,
			//		},
			//	},
			//},
			{
				Name:      "swipe",
				Aliases:   []string{"w"},
				Usage:     "updates Terraform module versions with versioned hashes",
				UsageText: "ghat swipe",
				Action: func(*cli.Context) error {
					return myFlags.Action("swipe")
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "file",
						Aliases:     []string{"f"},
						Usage:       "module file to parse",
						Destination: &myFlags.File,
						Category:    "files",
					},
					&cli.StringFlag{
						Name:        "directory",
						Aliases:     []string{"d"},
						Usage:       "Destination to update modules",
						Value:       ".",
						Destination: &myFlags.Directory,
						Category:    "files",
					},
					&cli.BoolFlag{
						Name:        "update",
						Usage:       "update to latest module available",
						Destination: &myFlags.Update,
						Value:       false,
					},
					&cli.BoolFlag{
						Name:        "dry-run",
						Usage:       "show but don't write changes",
						Destination: &myFlags.DryRun,
						Value:       false,
					},
					&cli.StringFlag{
						Name:        "token",
						Aliases:     []string{"t"},
						Usage:       "Github PAT token",
						Destination: &myFlags.GitHubToken,
						Category:    "authentication",
						EnvVars:     []string{"GITHUB_TOKEN", "GITHUB_API"},
					},
				},
			},
			{
				Name:      "sift",
				Aliases:   []string{"p"},
				Usage:     "updates pre-commit version with  hashes",
				UsageText: "ghat sift",
				Action: func(*cli.Context) error {
					return myFlags.Action("sift")
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "directory",
						Aliases:     []string{"d"},
						Usage:       "Destination to update modules",
						Destination: &myFlags.Directory,
						Category:    "files",
					},
					&cli.BoolFlag{
						Name:        "dry-run",
						Usage:       "show but don't write changes",
						Destination: &myFlags.DryRun,
						Value:       false,
					},
					&cli.StringFlag{
						Name:        "token",
						Aliases:     []string{"t"},
						Usage:       "Github PAT token",
						Destination: &myFlags.GitHubToken,
						Category:    "authentication",
						EnvVars:     []string{"GITHUB_TOKEN", "GITHUB_API"},
					},
				},
			},
			cacheCmd,
			swotCmd,
		},
		Name:     "ghat",
		Usage:    "Update GHA dependencies",
		Compiled: time.Time{},
		Authors:  []*cli.Author{{Name: "James Woolfenden", Email: "jim.wolf@duck.com"}},
		Version:  version.Version,
	}
	sort.Sort(cli.FlagsByName(app.Flags))
	sort.Sort(cli.CommandsByName(app.Commands))

	if err := app.Run(os.Args); err != nil {
		log.Fatal().Err(err).Msg("ghat failure")
	}
}

// Add these flags to your CLI commands (swot, swipe, sift)

// Example for the swot command:
var swotCmd = &cli.Command{
	Name:    "swot",
	Aliases: []string{"a"},
	Usage:   "updates GHA versions for hashes",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "directory",
			Aliases: []string{"d"},
			Usage:   "Directory to scan for workflow files",
		},
		&cli.StringFlag{
			Name:    "file",
			Aliases: []string{"f"},
			Usage:   "Specific workflow file to update",
		},
		&cli.BoolFlag{
			Name:  "dryrun",
			Usage: "Show changes without modifying files",
		},
		&cli.UintFlag{
			Name:  "stable",
			Usage: "Use releases from N days ago (more stable)",
		},
		// NEW CACHE FLAGS
		&cli.BoolFlag{
			Name:  "no-cache",
			Usage: "Disable caching of API responses",
		},
		&cli.DurationFlag{
			Name:  "cache-ttl",
			Usage: "Cache time-to-live (e.g., 24h, 1h30m)",
			Value: 24 * time.Hour,
		},
		&cli.BoolFlag{
			Name:  "clear-cache",
			Usage: "Clear the cache before running",
		},
	},
	Action: func(c *cli.Context) error {
		// Initialize flags
		myFlags := core.NewFlags()
		myFlags.Directory = c.String("directory")
		myFlags.File = c.String("file")
		myFlags.DryRun = c.Bool("dryrun")

		if c.IsSet("stable") {
			stable := c.Uint("stable")
			myFlags.Stable = &stable
		}

		// Cache configuration
		myFlags.CacheEnabled = !c.Bool("no-cache")
		myFlags.CacheTTL = c.Duration("cache-ttl")

		// Initialize cache
		if err := myFlags.InitializeCache(); err != nil {
			return fmt.Errorf("failed to initialize cache: %w", err)
		}

		// Clear cache if requested
		if c.Bool("clear-cache") && myFlags.Cache != nil {
			if err := myFlags.Cache.Clear(); err != nil {
				log.Warn().Err(err).Msg("Failed to clear cache")
			} else {
				log.Info().Msg("Cache cleared")
			}
		}

		// Show cache stats at the end
		defer func() {
			if myFlags.Cache != nil && myFlags.Cache.IsEnabled() {
				count, size, err := myFlags.Cache.Stats()
				if err == nil {
					log.Info().
						Int("entries", count).
						Int64("size_bytes", size).
						Msg("Cache statistics")
				}
			}
		}()

		// Your existing command logic here
		// ...

		return nil
	},
}

// Add similar flags to swipe and sift commands
var cacheCmd = &cli.Command{
	Name:  "cache",
	Usage: "Manage API response cache",
	Subcommands: []*cli.Command{
		{
			Name:  "clear",
			Usage: "Clear all cached entries",
			Action: func(c *cli.Context) error {
				cache, err := core.NewCache(24*time.Hour, true)
				if err != nil {
					return err
				}

				if err := cache.Clear(); err != nil {
					return fmt.Errorf("failed to clear cache: %w", err)
				}

				log.Info().Msg("✓ Cache cleared successfully")
				return nil
			},
		},
		{
			Name:  "stats",
			Usage: "Show cache statistics",
			Action: func(c *cli.Context) error {
				cache, err := core.NewCache(24*time.Hour, true)
				if err != nil {
					return err
				}

				count, size, err := cache.Stats()
				if err != nil {
					return fmt.Errorf("failed to get cache stats: %w", err)
				}

				fmt.Printf("📊 Cache Statistics:\n")
				fmt.Printf("   Entries: %d\n", count)
				fmt.Printf("   Size:    %s\n", formatBytes(size))

				return nil
			},
		},
		{
			Name:  "clean",
			Usage: "Remove expired cache entries",
			Action: func(c *cli.Context) error {
				cache, err := core.NewCache(24*time.Hour, true)
				if err != nil {
					return err
				}

				if err := cache.ClearExpired(); err != nil {
					return fmt.Errorf("failed to clean cache: %w", err)
				}

				count, size, err := cache.Stats()
				if err != nil {
					return fmt.Errorf("failed to get cache stats: %w", err)
				}

				log.Info().
					Int("entries", count).
					Int64("size_bytes", size).
					Msg("✓ Cache cleaned")

				return nil
			},
		},
	},
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
