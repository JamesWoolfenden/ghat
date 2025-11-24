package core

import (
	"os"
	"path/filepath"
)

const (
	ActionSwipe = "swipe"
	ActionSwot  = "swot"
	ActionSift  = "sift"
	ActionStun  = "stun"
	ActionShake = "shake"
)

func (myFlags *Flags) Action(action string) error {
	var err error

	if action == "" {
		return &actionIsEmptyError{}
	}

	if myFlags.File != "" {
		if _, err := os.Stat(myFlags.File); err != nil {
			pwd, err := os.Getwd()
			if err != nil {
				return &workingDirectoryError{pwd}
			}
			myFlags.File = filepath.Join(pwd, myFlags.File)
		}

		myFlags.Entries = append(myFlags.Entries, myFlags.File)
	} else {
		myFlags.Entries, err = GetFiles(myFlags.Directory)

		if err != nil {
			return &directoryReadError{myFlags.Directory}
		}
	}

	err = executeAction(action, myFlags)
	if err != nil {
		return &executeActionError{action}
	}

	return nil
}

func executeAction(action string, myFlags *Flags) error {
	if myFlags == nil {
		return &actionIsEmptyError{}
	}

	if myFlags.File == "" && myFlags.Directory == "" {
		return &dirAndFileEmptyError{}
	}

	switch action {
	case ActionSwipe:
		if myFlags.File != "" {
			return myFlags.UpdateModule(myFlags.File)
		} else {
			return myFlags.UpdateModules()
		}
	case ActionSwot:
		{
			if myFlags.File != "" {
				return myFlags.UpdateGHA(myFlags.File)
			} else {
				return myFlags.UpdateGHAS()
			}
		}
	case ActionSift:
		{
			return myFlags.UpdateHooks()
		}
	case ActionStun:
		{
			return myFlags.UpdateGitlab()
		}
	case ActionShake:
		{
			return myFlags.UpdateProviders()
		}
	}

	return nil
}
