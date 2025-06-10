package undoer

// Export internal functions for testing.
var ParseGitCommand = parseGitCommand

// Constructor functions for testing with private fields

func NewCherryPickUndoerForTest(git GitExec, originalCmd *CommandDetails) *CherryPickUndoer {
	return &CherryPickUndoer{
		git:         git,
		originalCmd: originalCmd,
	}
}

func NewCleanUndoerForTest(git GitExec, originalCmd *CommandDetails) *CleanUndoer {
	return &CleanUndoer{
		git:         git,
		originalCmd: originalCmd,
	}
}

func NewMvUndoerForTest(git GitExec, originalCmd *CommandDetails) *MvUndoer {
	return &MvUndoer{
		git:         git,
		originalCmd: originalCmd,
	}
}

func NewResetUndoerForTest(git GitExec, originalCmd *CommandDetails) *ResetUndoer {
	return &ResetUndoer{
		git:         git,
		originalCmd: originalCmd,
	}
}

func NewRestoreUndoerForTest(git GitExec, originalCmd *CommandDetails) *RestoreUndoer {
	return &RestoreUndoer{
		git:         git,
		originalCmd: originalCmd,
	}
}

func NewRevertUndoerForTest(git GitExec, originalCmd *CommandDetails) *RevertUndoer {
	return &RevertUndoer{
		git:         git,
		originalCmd: originalCmd,
	}
}

func NewRmUndoerForTest(git GitExec, originalCmd *CommandDetails) *RmUndoer {
	return &RmUndoer{
		git:         git,
		originalCmd: originalCmd,
	}
}

func NewSwitchUndoerForTest(git GitExec, originalCmd *CommandDetails) *SwitchUndoer {
	return &SwitchUndoer{
		git:         git,
		originalCmd: originalCmd,
	}
}

func NewTagUndoerForTest(git GitExec, originalCmd *CommandDetails) *TagUndoer {
	return &TagUndoer{
		git:         git,
		originalCmd: originalCmd,
	}
}
