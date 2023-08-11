package core

type Flags struct {
	File        string
	Directory   string
	GitHubToken string
	Days        int
	DryRun      bool
	Entries     []string
}
