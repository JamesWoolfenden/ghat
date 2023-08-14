package core

import (
	"fmt"
	"github.com/go-git/go-git/v5/storage/memory"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/rs/zerolog/log"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/zclconf/go-cty/cty"
	"golang.org/x/mod/semver"
)

func (myFlags *Flags) UpdateModule(file string) error {

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
			version := GetStringValue(block, "version")
			source := GetStringValue(block, "source")

			block.Body().RemoveAttribute("version")

			myType, err := myFlags.GetType(source)

			if err != nil {
				log.Info().Msgf("source type failure")
			} else {
				newValue, version, err := myFlags.UpdateSource(source, myType, version)
				if err != nil {
					log.Info().Msgf("failed to update module source %s", err)
				}
				block.Body().SetAttributeValue("source", cty.StringVal(newValue+"# "+version))
			}
		}

		newBody.AppendBlock(block)
	}

	var differ bool

	temp := outFile.Bytes()

	if string(src) != string(temp) {
		differ = true
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(string(src), string(temp), false)

	if differ {
		fmt.Println(dmp.DiffPrettyText(diffs))
	}

	if differ && !myFlags.DryRun {
		err := os.WriteFile(file, outFile.Bytes(), 0666)
		if err != nil {
			log.Info().Msgf("failed to write %s", file)
		}
	}

	return nil
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

	err = os.MkdirAll(module, 0700)

	if err == nil {
		_ = os.Remove(module)
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
			//var hash string
			newModule := strings.TrimLeft(module, "git::")

			if strings.Contains(newModule, ",") {
				commas := strings.Split(newModule, ",")

				if len(commas) > 1 {
					log.Info().Msgf("too many urlparams")
					//do something
				}
			}
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
					log.Print(ref)
				}

				// get latest hash for root
				log.Print(root)
			} else {
				if strings.Contains(newModule, "github.com") {
					if version != "" {
						hash, err = myFlags.GetGithubHash(strings.TrimLeft(newModule, "https://"), version)
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
				}

				log.Info().Msgf(" git != github")
			}
		}

	case "registry":
		{
			splits := strings.Split(module, "/")
			if len(splits) != 4 {
				return "", "", fmt.Errorf("registry format should split 3 ways")
			}

			newModule := splits[0] + "/" + splits[1] + "-" + splits[3] + "-" + splits[2]

			return myFlags.UpdateGithubSource(version, newModule)
		}

	case "github":
		{

		}

	case "mercurial":
		{

		}

	case "local", "shallow", "archive", "s3", "gcs":
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

	root := "github.com/" + newModule
	return "git::" + root + "?ref=" + hash, version, nil
}

func (myFlags *Flags) GetGithubLatestHash(newModule string) (string, string, error) {

	name := strings.Split(newModule, "github.com/")

	action := strings.Split(name[1], ".git")
	payload, err := GetLatest(action[0], myFlags.GitHubToken)

	if err != nil {
		return "", "", err
	}

	assertedPayload := payload.(map[string]interface{})

	version := assertedPayload["tag_name"].(string)
	hash, err := myFlags.GetGithubHash(newModule, version)

	if err != nil {
		return "", "", err
	}

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
			return "", err
		}

		assertedPayload := payload.(map[string]interface{})

		object := assertedPayload["object"].(map[string]interface{})

		hash = object["sha"].(string)
	} else {
		if len(tag) == 64 || len(tag) == 7 {
			hash = tag
		} else {
			return "", fmt.Errorf("supplied hash is not a short or a long hash")
		}
	}

	return hash, err
}
