package core

import (
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

func (myFlags *Flags) Action(Action string) error {
	var err error

	if myFlags.File != "" {
		if _, err := os.Stat(myFlags.File); err != nil {
			pwd, _ := os.Getwd()
			myFlags.File = filepath.Join(pwd, myFlags.File)
		}
	} else {
		myFlags.Entries, err = GetFiles(myFlags.Directory)

		if err != nil {
			log.Error().Msgf("failed to read %s", myFlags.Directory)
		}
	}

	switch Action {
	case "swipe":
		{
			if myFlags.File != "" {
				return myFlags.UpdateModule(myFlags.File)
			} else {
				return myFlags.UpdateModules()
			}
		}
	case "swot":
		{
			if myFlags.File != "" {
				return myFlags.UpdateGHA(myFlags.File)
			} else {
				return myFlags.UpdateGHAS()
			}
		}
	}

	return nil
}
