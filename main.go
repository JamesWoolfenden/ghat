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
			{
				Name:      "swot",
				Aliases:   []string{"a"},
				Usage:     "updates GHA versions for hashes",
				UsageText: "ghat swot",
				Action: func(*cli.Context) error {
					return myFlags.Action("swot")
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "file",
						Aliases:     []string{"f"},
						Usage:       "GHA file to parse",
						Destination: &myFlags.File,
						Category:    "files",
					},
					&cli.StringFlag{
						Name:        "directory",
						Aliases:     []string{"d"},
						Usage:       "Destination to update GHAs",
						Value:       ".",
						Destination: &myFlags.Directory,
						Category:    "files",
					},
					&cli.UintFlag{
						Name:        "stable",
						Aliases:     []string{"s"},
						Usage:       "days to wait for stabilisation of release",
						Value:       0,
						Destination: &myFlags.Days,
						DefaultText: "0",
						Category:    "delay",
					},
					&cli.StringFlag{
						Name:        "token",
						Aliases:     []string{"t"},
						Usage:       "Github PAT token",
						Destination: &myFlags.GitHubToken,
						Category:    "authentication",
						EnvVars:     []string{"GITHUB_TOKEN", "GITHUB_API"},
					},
					&cli.BoolFlag{
						Name:        "dry-run",
						Usage:       "show but don't write changes",
						Destination: &myFlags.DryRun,
						Value:       false,
					},
					&cli.BoolFlag{
						Name:        "continue-on-error",
						Usage:       "just keep going",
						Destination: &myFlags.ContinueOnError,
						Value:       false,
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
