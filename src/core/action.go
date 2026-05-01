package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// label tags a sweep sub-action error with its verb so errors.Join output is attributable.
func label(verb string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", verb, err)
}

const (
	ActionSwipe = "swipe"
	ActionSwot  = "swot"
	ActionSift  = "sift"
	ActionStun  = "stun"
	ActionShake = "shake"
	ActionKube  = "kube"
	ActionDock  = "dock"
	ActionSweep = "sweep"
	ActionSub   = "sub"
	ActionAudit = "audit"
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
		return &executeActionError{action: action, err: err}
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
	case ActionKube:
		if myFlags.File != "" {
			return myFlags.UpdateKube(myFlags.File)
		}
		return myFlags.UpdateKubes()
	case ActionDock:
		if myFlags.File != "" {
			return myFlags.UpdateDockerfile(myFlags.File)
		}
		return myFlags.UpdateDockerfiles()
	case ActionSub:
		return myFlags.UpdateSubmodules()
	case ActionAudit:
		return myFlags.Audit()
	case ActionSweep:
		return errors.Join(
			label(ActionSwot, myFlags.UpdateGHAS()),
			label(ActionStun, myFlags.UpdateGitlab()),
			label(ActionSift, myFlags.UpdateHooks()),
			label(ActionSwipe, myFlags.UpdateModules()),
			label(ActionShake, myFlags.UpdateProviders()),
			label(ActionKube, myFlags.UpdateKubes()),
			label(ActionDock, myFlags.UpdateDockerfiles()),
			label(ActionSub, myFlags.UpdateSubmodules()),
			label("cpan", myFlags.UpdateCpanfile()),
		)
	}

	return nil
}
