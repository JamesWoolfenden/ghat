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
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	var myFlags core.Flags

	app := &cli.App{
		EnableBashCompletion: true,
		Copyright:            "James Woolfenden",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "quiet",
				Usage: "suppress banner and log output (useful in pre-commit hooks)",
			},
		},
		Before: func(c *cli.Context) error {
			if !c.Bool("quiet") {
				fmt.Println(banner.Inline("ghat"))
				fmt.Println("version:", version.Version)
			} else {
				log.Logger = zerolog.Nop()
			}
			return nil
		},
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
			{
				Name:      "stun",
				Aliases:   []string{"t"},
				Usage:     "updates Gitlab versions for hashes",
				UsageText: "ghat stun",
				Action: func(c *cli.Context) error {
					myFlags.Directory = c.String("directory")
					myFlags.DryRun = c.Bool("dry-run")

					if c.IsSet("stable") {
						stable := c.Uint("stable")
						myFlags.Days = &stable
					}

					return myFlags.Action("stun")
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "directory",
						Aliases:  []string{"d"},
						Usage:    "repository Destination",
						Value:    ".",
						Category: "files",
					},
					&cli.UintFlag{
						Name:        "stable",
						Aliases:     []string{"s"},
						Usage:       "days to wait for stabilisation of release",
						Value:       0,
						DefaultText: "0",
						Category:    "delay",
					},
					&cli.BoolFlag{
						Name:  "dry-run",
						Usage: "show but don't write changes",
						Value: false,
					},
				},
			},
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
				Usage:     "updates pre-commit version with hashes",
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
			shakeCmd,
			cacheCmd,
			swotCmd,
			kubeCmd,
			dockCmd,
			subCmd,
			sweepCmd,
			auditCmd,
			orgCmd,
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
			Name:  "dry-run",
			Usage: "Show changes without modifying files",
		},
		&cli.BoolFlag{
			Name:  "continue-on-error",
			Usage: "Continue processing files even if errors occur",
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
		&cli.BoolFlag{
			Name:  "pin-only",
			Usage: "pin current tag to SHA without checking for upgrades",
		},
	},
	Action: func(c *cli.Context) error {
		myFlags := core.NewFlags()
		myFlags.Directory = c.String("directory")
		myFlags.File = c.String("file")
		myFlags.DryRun = c.Bool("dry-run")
		myFlags.ContinueOnError = c.Bool("continue-on-error")
		myFlags.PinOnly = c.Bool("pin-only")
		myFlags.GitHubToken = githubToken()

		stable := c.Uint("stable")
		myFlags.Days = &stable

		myFlags.CacheEnabled = !c.Bool("no-cache")
		myFlags.CacheTTL = c.Duration("cache-ttl")

		if err := myFlags.InitializeCache(); err != nil {
			return fmt.Errorf("failed to initialize cache: %w", err)
		}

		if c.Bool("clear-cache") && myFlags.Cache != nil {
			if err := myFlags.Cache.Clear(); err != nil {
				log.Warn().Err(err).Msg("Failed to clear cache")
			} else {
				log.Info().Msg("Cache cleared")
			}
		}

		defer func() {
			if myFlags.Cache != nil && myFlags.Cache.IsEnabled() {
				count, size, err := myFlags.Cache.Stats()
				if err == nil {
					log.Info().Int("entries", count).Int64("size_bytes", size).Msg("Cache statistics")
				}
			}
		}()

		return myFlags.Action("swot")
	},
}

// Add similar flags to swipe and sift commands
var shakeCmd = &cli.Command{
	Name:    "shake",
	Aliases: []string{"k"},
	Usage:   "updates Terraform provider versions to latest",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "directory",
			Aliases: []string{"d"},
			Usage:   "Directory to scan for Terraform files",
			Value:   ".",
		},
		&cli.StringFlag{
			Name:    "file",
			Aliases: []string{"f"},
			Usage:   "Specific Terraform file to update",
		},
		&cli.BoolFlag{
			Name:  "dry-run",
			Usage: "Show changes without modifying files",
		},
		&cli.BoolFlag{
			Name:  "continue-on-error",
			Usage: "Continue processing files even if errors occur",
		},
	},
	Action: func(c *cli.Context) error {
		myFlags := core.NewFlags()
		myFlags.Directory = c.String("directory")
		myFlags.File = c.String("file")
		myFlags.DryRun = c.Bool("dry-run")
		myFlags.ContinueOnError = c.Bool("continue-on-error")

		return myFlags.Action("shake")
	},
}

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

var kubeCmd = &cli.Command{
	Name:    "kube",
	Aliases: []string{"k8s"},
	Usage:   "pins container images in Kubernetes manifests to SHA digests",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "directory",
			Aliases: []string{"d"},
			Usage:   "directory to scan for Kubernetes manifests",
			Value:   ".",
		},
		&cli.StringFlag{
			Name:    "file",
			Aliases: []string{"f"},
			Usage:   "specific manifest file to update",
		},
		&cli.BoolFlag{
			Name:  "dry-run",
			Usage: "show changes without modifying files",
		},
		&cli.BoolFlag{
			Name:  "continue-on-error",
			Usage: "continue processing files even if errors occur",
		},
	},
	Action: func(c *cli.Context) error {
		myFlags := core.NewFlags()
		myFlags.Directory = c.String("directory")
		myFlags.File = c.String("file")
		myFlags.DryRun = c.Bool("dry-run")
		myFlags.ContinueOnError = c.Bool("continue-on-error")
		myFlags.GitHubToken = githubToken()

		if myFlags.File == "" {
			var err error
			myFlags.Entries, err = core.GetFiles(myFlags.Directory)
			if err != nil {
				return fmt.Errorf("failed to scan directory: %w", err)
			}
		}

		return myFlags.Action("kube")
	},
}

var dockCmd = &cli.Command{
	Name:    "dock",
	Aliases: []string{"df"},
	Usage:   "pins Dockerfile FROM images to SHA digests",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "directory",
			Aliases: []string{"d"},
			Usage:   "directory to scan for Dockerfiles",
			Value:   ".",
		},
		&cli.StringFlag{
			Name:    "file",
			Aliases: []string{"f"},
			Usage:   "specific Dockerfile to update",
		},
		&cli.BoolFlag{
			Name:  "dry-run",
			Usage: "show changes without modifying files",
		},
		&cli.BoolFlag{
			Name:  "continue-on-error",
			Usage: "continue processing files even if errors occur",
		},
	},
	Action: func(c *cli.Context) error {
		myFlags := core.NewFlags()
		myFlags.Directory = c.String("directory")
		myFlags.File = c.String("file")
		myFlags.DryRun = c.Bool("dry-run")
		myFlags.ContinueOnError = c.Bool("continue-on-error")
		myFlags.GitHubToken = githubToken()

		if myFlags.File == "" {
			var err error
			myFlags.Entries, err = core.GetFiles(myFlags.Directory)
			if err != nil {
				return fmt.Errorf("failed to scan directory: %w", err)
			}
		}

		return myFlags.Action("dock")
	},
}

var subCmd = &cli.Command{
	Name:      "sub",
	Aliases:   []string{"m"},
	Usage:     "updates git submodule pins to latest tagged release SHA",
	UsageText: "ghat sub -d .",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "directory",
			Aliases: []string{"d"},
			Usage:   "repository containing .gitmodules",
			Value:   ".",
		},
		&cli.BoolFlag{
			Name:  "dry-run",
			Usage: "show changes without modifying the index",
		},
		&cli.BoolFlag{
			Name:  "continue-on-error",
			Usage: "continue processing submodules even if one fails",
		},
		&cli.StringFlag{
			Name:     "token",
			Aliases:  []string{"t"},
			Usage:    "GitHub PAT token",
			Category: "authentication",
			EnvVars:  []string{"GITHUB_TOKEN", "GITHUB_API"},
		},
	},
	Action: func(c *cli.Context) error {
		myFlags := core.NewFlags()
		myFlags.Directory = c.String("directory")
		myFlags.DryRun = c.Bool("dry-run")
		myFlags.ContinueOnError = c.Bool("continue-on-error")
		myFlags.GitHubToken = c.String("token")

		return myFlags.Action(core.ActionSub)
	},
}

var sweepCmd = &cli.Command{
	Name:      "all",
	Aliases:   []string{"sweep"},
	Usage:     "runs every pinner (GHA, GitLab, pre-commit, Terraform, Kubernetes, Dockerfiles) against a directory",
	UsageText: "ghat all -d .",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "directory",
			Aliases: []string{"d"},
			Usage:   "directory to scan",
			Value:   ".",
		},
		&cli.BoolFlag{
			Name:  "dry-run",
			Usage: "show changes without modifying files",
		},
		&cli.BoolFlag{
			Name:  "continue-on-error",
			Usage: "continue processing files even if errors occur",
		},
		&cli.UintFlag{
			Name:  "stable",
			Usage: "use releases from N days ago (more stable)",
		},
		&cli.BoolFlag{
			Name:  "update",
			Usage: "update Terraform modules to latest available",
		},
		&cli.StringFlag{
			Name:     "token",
			Aliases:  []string{"t"},
			Usage:    "GitHub PAT token",
			Category: "authentication",
			EnvVars:  []string{"GITHUB_TOKEN", "GITHUB_API"},
		},
		&cli.BoolFlag{
			Name:  "no-cache",
			Usage: "disable caching of API responses",
		},
		&cli.DurationFlag{
			Name:  "cache-ttl",
			Usage: "cache time-to-live (e.g., 24h, 1h30m)",
			Value: 24 * time.Hour,
		},
		&cli.BoolFlag{
			Name:  "pin-only",
			Usage: "pin current tag to SHA without checking for upgrades",
		},
	},
	Action: func(c *cli.Context) error {
		myFlags := core.NewFlags()
		myFlags.Directory = c.String("directory")
		myFlags.DryRun = c.Bool("dry-run")
		myFlags.ContinueOnError = c.Bool("continue-on-error")
		myFlags.Update = c.Bool("update")
		myFlags.PinOnly = c.Bool("pin-only")
		myFlags.GitHubToken = c.String("token")

		stable := c.Uint("stable")
		myFlags.Days = &stable

		myFlags.CacheEnabled = !c.Bool("no-cache")
		myFlags.CacheTTL = c.Duration("cache-ttl")

		if err := myFlags.InitializeCache(); err != nil {
			return fmt.Errorf("failed to initialize cache: %w", err)
		}

		return myFlags.Action(core.ActionSweep)
	},
}

var auditCmd = &cli.Command{
	Name:      "audit",
	Aliases:   []string{"sc"},
	Usage:     "scores your dependencies (go.mod, GHA uses:, pre-commit, Terraform modules) by whether their CI workflows pin actions to SHAs",
	UsageText: "ghat audit -d . [--source go,gha,pre-commit,terraform] [--deep]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "directory",
			Aliases: []string{"d"},
			Usage:   "directory to scan for dependency manifests",
			Value:   ".",
		},
		&cli.BoolFlag{
			Name:  "deep",
			Usage: "audit transitive Go dependencies (go list -m all) instead of direct only",
		},
		&cli.StringSliceFlag{
			Name:    "source",
			Aliases: []string{"s"},
			Usage:   "dependency sources to audit (go, gha, pre-commit, terraform); repeat or comma-separate; default all",
		},
		&cli.StringFlag{
			Name:     "token",
			Aliases:  []string{"t"},
			Usage:    "GitHub PAT token",
			Category: "authentication",
			EnvVars:  []string{"GITHUB_TOKEN", "GITHUB_API"},
		},
		&cli.BoolFlag{
			Name:  "no-cache",
			Usage: "disable caching of API responses",
		},
		&cli.DurationFlag{
			Name:  "cache-ttl",
			Usage: "cache time-to-live (e.g., 24h, 1h30m)",
			Value: 24 * time.Hour,
		},
	},
	Action: func(c *cli.Context) error {
		myFlags := core.NewFlags()
		myFlags.Directory = c.String("directory")
		myFlags.Deep = c.Bool("deep")
		myFlags.Sources = c.StringSlice("source")
		myFlags.GitHubToken = c.String("token")
		myFlags.CacheEnabled = !c.Bool("no-cache")
		myFlags.CacheTTL = c.Duration("cache-ttl")

		if err := myFlags.InitializeCache(); err != nil {
			return fmt.Errorf("failed to initialize cache: %w", err)
		}

		return myFlags.Action(core.ActionAudit)
	},
}

// githubToken returns the first non-empty value of GITHUB_TOKEN or GITHUB_API.
var orgCmd = &cli.Command{
	Name:      "org",
	Usage:     "run ghat all across every non-fork repo for a GitHub/GitLab user, org or group",
	UsageText: "ghat org [--provider github|gitlab] [--owner name] [--limit 10] [--dry-run] [--pr]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "provider",
			Usage: "code host: github or gitlab",
			Value: "github",
		},
		&cli.StringFlag{
			Name:  "base-url",
			Usage: "self-hosted API root (e.g. https://gitlab.example.com)",
		},
		&cli.StringFlag{
			Name:    "owner",
			Aliases: []string{"o"},
			Usage:   "user, org, or GitLab group (default: authenticated user)",
		},
		&cli.StringSliceFlag{
			Name:    "repo",
			Aliases: []string{"r"},
			Usage:   "target a specific repo (owner/name); repeat for multiple; skips listing",
		},
		&cli.IntFlag{
			Name:  "offset",
			Usage: "skip the first N repos",
			Value: 0,
		},
		&cli.IntFlag{
			Name:  "limit",
			Usage: "stop after N repos (0 = all)",
			Value: 0,
		},
		&cli.BoolFlag{
			Name:  "dry-run",
			Usage: "show changes without writing or opening PRs",
		},
		&cli.BoolFlag{
			Name:  "pr",
			Usage: "open a pull request for each repo with changes",
		},
		&cli.BoolFlag{
			Name:  "auto-merge",
			Usage: "enable auto-merge on each PR (requires repo setting to be on)",
		},
		&cli.StringFlag{
			Name:     "token",
			Aliases:  []string{"t"},
			Usage:    "PAT for the chosen provider",
			Category: "authentication",
			EnvVars:  []string{"GITHUB_TOKEN", "GITHUB_API", "GITLAB_TOKEN"},
		},
		&cli.StringFlag{
			Name:  "branch",
			Usage: "branch name for pinning PRs",
			Value: "ghat/pin-dependencies",
		},
		&cli.IntFlag{
			Name:  "rate-threshold",
			Usage: "pause when fewer than N API requests remain",
			Value: 200,
		},
	},
	Action: func(c *cli.Context) error {
		flags := &core.OrgFlags{
			Provider:  c.String("provider"),
			BaseURL:   c.String("base-url"),
			Owner:     c.String("owner"),
			Repos:     c.StringSlice("repo"),
			Token:     c.String("token"),
			Branch:    c.String("branch"),
			Offset:    c.Int("offset"),
			Limit:     c.Int("limit"),
			DryRun:    c.Bool("dry-run"),
			OpenPR:    c.Bool("pr"),
			AutoMerge: c.Bool("auto-merge"),
			Threshold: c.Int("rate-threshold"),
		}
		if flags.Token == "" {
			flags.Token = githubToken()
		}

		results, err := flags.RunBulk()
		if err != nil {
			return err
		}

		var pinned, already, prOpen, errors int
		for _, r := range results {
			switch r.Status {
			case "pinned":
				pinned++
				if r.PRUrl != "" {
					fmt.Printf("  PR  %s\n", r.PRUrl)
				}
			case "already-pinned":
				already++
			case "pr-open":
				prOpen++
				if r.PRUrl != "" {
					fmt.Printf("  PR  %s\n", r.PRUrl)
				}
			case "error":
				errors++
				fmt.Fprintf(os.Stderr, "  ERR %s: %v\n", r.Repo, r.Error)
			}
			if len(r.Gaps) > 0 {
				for _, g := range r.Gaps {
					fmt.Fprintf(os.Stderr, "  GAP %s\n", g)
				}
			}
		}

		fmt.Printf("\n%d pinned, %d already clean, %d PR already open, %d errors\n",
			pinned, already, prOpen, errors)
		return nil
	},
}

func githubToken() string {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	return os.Getenv("GITHUB_API")
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
