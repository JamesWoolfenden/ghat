package core

type Flags struct {
	File            string
	Directory       string
	GitHubToken     string
	Days            uint
	DryRun          bool
	Entries         []string
	Update          bool
	ContinueOnError bool
}
