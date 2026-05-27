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

func (f *Flags) Action(action string) error {
	var err error

	if action == "" {
		return &actionIsEmptyError{}
	}

	if f.File != "" {
		if _, err := os.Stat(f.File); err != nil {
			pwd, err := os.Getwd()
			if err != nil {
				return &workingDirectoryError{pwd}
			}
			f.File = filepath.Join(pwd, f.File)
		}

		f.Entries = append(f.Entries, f.File)
	} else {
		f.Entries, err = GetFiles(f.Directory)

		if err != nil {
			return &directoryReadError{f.Directory}
		}
	}

	err = executeAction(action, f)
	if err != nil {
		return &executeActionError{action: action, err: err}
	}

	return nil
}

func executeAction(action string, f *Flags) error {
	if f == nil {
		return &actionIsEmptyError{}
	}

	if f.File == "" && f.Directory == "" {
		return &dirAndFileEmptyError{}
	}

	switch action {
	case ActionSwipe:
		if f.File != "" {
			return f.UpdateModule(f.File)
		}

		return f.UpdateModules()
	case ActionSwot:
		{
			if f.File != "" {
				return f.UpdateGHA(f.File)
			}

			return f.UpdateGHAS()
		}
	case ActionSift:
		{
			return f.UpdateHooks()
		}
	case ActionStun:
		{
			return f.UpdateGitlab()
		}
	case ActionShake:
		{
			return f.UpdateProviders()
		}
	case ActionKube:
		if f.File != "" {
			return f.UpdateKube(f.File)
		}
		return f.UpdateKubes()
	case ActionDock:
		if f.File != "" {
			return f.UpdateDockerfile(f.File)
		}
		return f.UpdateDockerfiles()
	case ActionSub:
		return f.UpdateSubmodules()
	case ActionAudit:
		return f.Audit()
	case ActionSweep:
		return errors.Join(
			label(ActionSwot, f.UpdateGHAS()),
			label(ActionStun, f.UpdateGitlab()),
			label(ActionSift, f.UpdateHooks()),
			label(ActionSwipe, f.UpdateModules()),
			label(ActionShake, f.UpdateProviders()),
			label(ActionKube, f.UpdateKubes()),
			label(ActionDock, f.UpdateDockerfiles()),
			label(ActionSub, f.UpdateSubmodules()),
			label("cpan", f.UpdateCpanfile()),
		)
	}

	return nil
}
