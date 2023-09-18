package core

import (
	"fmt"
	"os"
	"path/filepath"
)

func (myFlags *Flags) Action(Action string) error {
	var err error

	if myFlags.File != "" {
		if _, err := os.Stat(myFlags.File); err != nil {
			pwd, _ := os.Getwd()
			myFlags.File = filepath.Join(pwd, myFlags.File)
		}

		myFlags.Entries = append(myFlags.Entries, myFlags.File)
	} else {
		myFlags.Entries, err = GetFiles(myFlags.Directory)

		if err != nil {
			return fmt.Errorf("action failed to read %s", myFlags.Directory)
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
