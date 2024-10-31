package core

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/rs/zerolog/log"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/zclconf/go-cty/cty"
	"golang.org/x/mod/semver"
)

func (myFlags *Flags) UpdateModule(file string) error {

	var version string
	var newValue string

	src, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read %s", file)
	}

	inFile, _ := hclwrite.ParseConfig(src, "", hcl.Pos{Line: 1, Column: 1})
	outFile := hclwrite.NewEmptyFile()

	newBody := outFile.Body()
	root := inFile.Body()

	for _, block := range root.Blocks() {
		if block.Type() == "module" {
			version = GetVersion(block)

			source := GetStringValue(block, "source")

			block.Body().RemoveAttribute("version")

			myType, err := myFlags.GetType(source)

			if err != nil {
				log.Info().Msgf("source type failure %s", source)
			} else {
				newValue, version, err = myFlags.UpdateSource(source, myType, version)
				if err != nil {
					log.Info().Msgf("failed to update module source %s", err)
				}
				block.Body().SetAttributeValue("source", cty.StringVal(newValue))
			}
		}

		newBody.AppendBlock(block)
	}

	var differ bool

	temp := string(outFile.Bytes())

	if version != "" {
		find := "\"" + newValue + "\""
		replacement := "  source = " + find + " #" + version

		lines := strings.Split(temp, "\n")

		for i, line := range lines {
			if strings.Contains(line, find) {
				lines[i] = replacement
				break
			}
		}

		temp = strings.Join(lines, "\n")
	}

	if string(src) != temp {
		differ = true
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(string(src), temp, false)

	if differ {
		fmt.Println(dmp.DiffPrettyText(diffs))
	}

	if differ && !myFlags.DryRun {
		err := os.WriteFile(file, []byte(temp), 0666)
		if err != nil {
			log.Info().Msgf("failed to write %s", file)
		}
	}

	return nil
}

func GetVersion(block *hclwrite.Block) string {
	version := GetStringValue(block, "version")
	if version == "" {
		return ""
	}

	constraints := []string{"=", "!", ">", ">", "~"}

	for _, constraint := range constraints {
		if strings.Contains(version, constraint) {
			version = ""
			log.Info().Msg("constraints not valid, using latest")
			continue
		}
	}

	if !strings.Contains(version, "v") && version != "" {
		version = "v" + version
	}

	return version
}

func GetStringValue(block *hclwrite.Block, attribute string) string {
	var Value string
	version := block.Body().GetAttribute(attribute)

	if (version != nil) && (len(version.Expr().BuildTokens(nil)) == 3) {
		Value = string(version.Expr().BuildTokens(nil)[1].Bytes)
	}
	return Value
}

func (myFlags *Flags) UpdateModules() error {

	terraform, err := myFlags.GetTF()

	if err != nil {
		return err
	}

	// contains a module?
	for _, file := range terraform {
		err = myFlags.UpdateModule(file)
		if err != nil {
			return err
		}
	}

	return nil
}

func (myFlags *Flags) GetTF() ([]string, error) {
	var terraform []string

	for _, match := range myFlags.Entries {
		//for each file that is a terraform file
		if path.Ext(match) == ".tf" {
			terraform = append(terraform, match)
		}
	}

	return terraform, nil
}

func (myFlags *Flags) GetType(module string) (string, error) {
	var moduleType string

	// handle local path
	absPath, _ := filepath.Abs(module)
	_, err := os.Stat(absPath)

	if err == nil {
		return "local", nil
	}

	if strings.Contains(module, "bitbucket.org") {
		return "bitbucket", nil
	}

	if strings.Contains(module, "s3::") {
		return "s3", nil
	}

	if strings.Contains(module, "gcs::") {
		return "gcs", nil
	}

	if strings.Contains(module, ".zip") || strings.Contains(module, "archive=") {
		return "archive", nil
	}

	// gitHub registry format and sub dirs
	splitter := strings.Split(module, "/")

	if len(splitter) == 3 && !(strings.Contains(module, "git::") || strings.Contains(module, "https:")) {
		if strings.Contains(module, "github.com") {
			return "github", nil
		}

		return "registry", nil
	}

	if strings.Contains(module, "depth=") {
		return "shallow", nil
	}

	if strings.Contains(module, "git::") {
		return "git", nil
	}

	if strings.Contains(module, "hg::") {
		return "mercurial", nil
	}

	if strings.Contains(module, "//") {
		temp := strings.Split(module, "//")[0]
		return myFlags.GetType(temp)
	}

	if _, err := os.Stat(module); os.IsNotExist(err) {
		return "local", fmt.Errorf("localpath not found %s", module)
	}

	return moduleType, err
}

func (myFlags *Flags) UpdateSource(module string, moduleType string, version string) (string, string, error) {

	var newModule string

	var hash string

	var err error

	switch moduleType {
	case "git":
		{
			newModule := strings.TrimPrefix(module, "git::")

			splitter := strings.Split(newModule, "?ref=")

			root := splitter[0]

			if len(splitter) > 1 {
				version = splitter[1]
			}

			if myFlags.Update {
				if strings.Contains(newModule, "github.com") {
					hash, version, err := myFlags.GetGithubLatestHash(newModule)
					if err != nil {
						return "", "", err
					}

					return "git::" + root + "?ref=" + hash, version, nil
				} else {
					repo, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
						URL: strings.TrimRight(module, ".git"),
					})

					if err != nil {
						return "", "", fmt.Errorf("failed to clone %s", newModule)
					}

					ref, err := repo.Head()
					if err != nil {
						return "", "", err
					}
					log.Print(ref)
				}

				// get latest hash for root
				log.Print(root)
			} else {
				if strings.Contains(newModule, "github.com") {
					if version != "" {
						hash, err = myFlags.GetGithubHash(
							strings.TrimPrefix(newModule, "https://"),
							version,
						)
						if err != nil {
							return "", "", err
						}
					} else {
						hash, version, err = myFlags.GetGithubLatestHash(newModule)
						if err != nil {
							return "", "", err
						}
					}
					return "git::" + root + "?ref=" + hash, version, nil
				} else {
					log.Info().Msgf("git != github")
				}
			}
		}

	case "registry":
		{
			var subDir string

			subDirs := strings.Split(module, "//")

			if len(subDirs) == 2 {
				subDir = subDirs[1]
				module = subDirs[0]
			}

			splits := strings.Split(module, "/")

			if len(splits) != 3 {
				return "", "", fmt.Errorf("registry format should split 3 ways")
			}

			//e.g. jameswoolfenden/terraform-http-ip
			newModule := "github.com" + "/" + splits[0] + "/" + "terraform" + "-" + splits[2] + "-" + splits[1] + ".git"

			if subDir == "" {
				return myFlags.UpdateGithubSource(version, newModule)
			} else {
				return myFlags.WithSubDir(version, newModule, subDir)
			}
		}

	case "github":
		{
			subDirs := strings.Split(module, "//")
			if len(subDirs) == 2 {
				subDir := subDirs[1]
				root := subDirs[0]

				// e.g. jameswoolfenden/terraform-http-ip
				newModule := root + ".git"

				return myFlags.WithSubDir(version, newModule, subDir)
			}

			newModule = module + ".git"
			return myFlags.UpdateGithubSource(version, newModule)
		}

	case "local", "shallow", "archive", "s3", "gcs", "mercurial":
		{
			log.Info().Msgf("module source is %s of type %s and cannot be updated", module, moduleType)
			return module, version, nil
		}

	default:
		{
			log.Info().Msgf("unknown module type encountered %s", moduleType)
		}
	}

	return newModule, version, nil
}

func (myFlags *Flags) WithSubDir(version string, newModule string, subdir string) (string, string, error) {
	url, version, err := myFlags.UpdateGithubSource(version, newModule)

	urlsplit := strings.Split(url, ".git")
	newUrl := urlsplit[0] + ".git" + "//" + subdir + urlsplit[1]

	return newUrl, version, err
}

func (myFlags *Flags) UpdateGithubSource(version string, newModule string) (string, string, error) {
	var hash string

	var err error

	if myFlags.Update {
		hash, version, err = myFlags.GetGithubLatestHash(newModule)
		if err != nil {
			return "", "", err
		}
	} else {
		if version != "" {
			hash, err = myFlags.GetGithubHash(newModule, version)
			if err != nil {
				return "", "", err
			}
		} else {
			hash, version, err = myFlags.GetGithubLatestHash(newModule)
			if err != nil {
				return "", "", err
			}
		}
	}

	return "git::https://" + newModule + "?ref=" + hash, version, nil
}

func (myFlags *Flags) GetGithubLatestHash(newModule string) (string, string, error) {
	name := strings.Split(newModule, "github.com/")

	if len(name) < 2 {
		return "", "", fmt.Errorf("modules string doesnt contain github.com")
	}

	action := strings.Split(name[1], ".git")
	if len(action) < 2 {
		return "", "", fmt.Errorf("modules string doesnt end in .git")
	}

	payload, err := GetLatestTag(action[0], myFlags.GitHubToken)

	if err != nil {
		return "", "", err
	}

	assertedPayload, ok := payload.(map[string]interface{})

	if !ok {
		return "", "", fmt.Errorf("type assertion failed")
	}

	version, ok := assertedPayload["name"].(string)

	if !ok {
		return "", "", fmt.Errorf("type assertion failed")
	}

	commit := assertedPayload["commit"].(map[string]interface{})

	hash := commit["sha"].(string)

	return hash, version, nil
}

func (myFlags *Flags) GetGithubHash(newModule string, tag string) (string, error) {
	var err error

	var hash string

	var url string

	var payload interface{}

	name := strings.Split(newModule, "github.com/")
	action := strings.Split(name[1], ".git")

	valid := semver.IsValid(tag)

	if valid {
		url = "https://api.github.com/repos/" + action[0] + "/git/ref/tags/" + tag
		payload, err = GetGithubBody(myFlags.GitHubToken, url)

		if err != nil {
			// retry as version is truncated
			if strings.Count(tag, ".") == 1 {
				tag = tag + ".0"
				url = "https://api.github.com/repos/" + action[0] + "/git/ref/tags/" + tag
				payload, err = GetGithubBody(myFlags.GitHubToken, url)
				if err != nil {
					log.Info().Msgf("failed to find tag %s", tag)
					return "", err
				}
			} else {
				return "", err
			}
		} else {
			log.Info().Msgf("failed to understand %s", tag)
		}

		assertedPayload := payload.(map[string]interface{})

		object := assertedPayload["object"].(map[string]interface{})

		hash = object["sha"].(string)
	} else {
		if len(tag) == 40 || len(tag) == 7 {
			hash = tag
		} else {
			return "", fmt.Errorf("supplied hash is not a short or a long hash")
		}
	}

	return hash, err
}
