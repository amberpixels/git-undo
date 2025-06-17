package undoer

import "fmt"

// InvalidUndoer represents an undoer for commands that cannot be parsed or are not supported.
type InvalidUndoer struct {
	rawCommand string
	parseError error
}

// GetUndoCommands implements the Undoer interface.
func (i *InvalidUndoer) GetUndoCommands() ([]*UndoCommand, error) {
	if i.parseError != nil {
		return nil, fmt.Errorf("%w: %w", ErrUndoNotSupported, i.parseError)
	}

	return nil, fmt.Errorf("%w: %s", ErrUndoNotSupported, i.rawCommand)
}
