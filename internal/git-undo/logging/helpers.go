package logging

import (
	"fmt"
	"os"
)

// EnsureLogDir ensures the git-undo log directory exists.
func EnsureLogDir(logDir string) error {
	// Creating LOG directory
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}
	return nil
}
