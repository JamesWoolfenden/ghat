package main

import (
	"fmt"
	"ghat/src/core"
	"ghat/src/version"
	"os"
	"sort"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

func main() {
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
				Usage:     "updates GHA in a directory",
				UsageText: "ghat swot",
				Action: func(*cli.Context) error {

					if myFlags.File != "" {
						err := myFlags.UpdateFile()
						if err != nil {
							return err
						}
					} else {
						_, err := myFlags.Files()
						if err != nil {
							return err
						}
					}

					return nil
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
					&cli.IntFlag{
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
