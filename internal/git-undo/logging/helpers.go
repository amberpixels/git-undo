package logging

import (
	"fmt"
	"os"
)

// EnsureLogDir ensures the git-undo log directory exists.
func EnsureLogDir(logDir string) error {

	// Creating LOG directory
	fmt.Println("CREATING LOG DIR", logDir)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}
	return nil
}
