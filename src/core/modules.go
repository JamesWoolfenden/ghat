package core

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/rs/zerolog/log"
	diff "github.com/yudai/gojsondiff"
	"github.com/zclconf/go-cty/cty"
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
				continue
			}

			block.Body().SetAttributeValue("source", cty.StringVal(myFlags.UpdateSource(source, myType, version)))
		}
		newBody.AppendBlock(block)
	}

	differ := diff.New()
	compare, err := differ.Compare(src, outFile.Bytes())

	if err != nil {
		return err
	}

	if compare.Modified() && !myFlags.DryRun {
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
		return "github", nil
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

func (myFlags *Flags) UpdateSource(module string, moduleType string, version string) string {
	//if strings.Contains("?ref=", module) {
	//	moduleType = "url"
	//
	//	if myFlags.Update {
	//		splitter := strings.Split(module, "?ref=")
	//		base := splitter[0]
	//		log.Print(base)
	//		// get lastest tag from git reference
	//		//trim git:
	//		//trim
	//	}
	//
	//	return moduleType, nil
	//}
	return "test"
}
