package logging

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

// EnsureLogDir ensures the git-undo log directory exists.
func EnsureLogDir(logDir string) error {
	// Creating LOG directory
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}
	return nil
}

// toggleLine toggles the line number in the log file.
func toggleLine(file *os.File, lineNumber int) error {
	// Reset to start of file
	_, err := file.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	reader := bufio.NewReader(file)
	var buffer bytes.Buffer
	currentLine := 0

	for {
		// Read the line including the newline character
		line, err := reader.ReadString('\n')
		isEOF := err == io.EOF

		if err != nil && !isEOF {
			return err
		}

		if currentLine == lineNumber {
			// Toggle the line
			if strings.HasPrefix(line, "#") {
				line = strings.TrimPrefix(line, "#")
			} else {
				line = "#" + line
			}
		}

		// Write the line to buffer
		buffer.WriteString(line)

		if isEOF {
			break
		}

		currentLine++
	}

	if currentLine < lineNumber {
		return fmt.Errorf("line %d not found: file has only %d lines", lineNumber, currentLine)
	}

	// Write back to file
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	err = file.Truncate(0)
	if err != nil {
		return err
	}

	_, err = buffer.WriteTo(file)
	return err
}
