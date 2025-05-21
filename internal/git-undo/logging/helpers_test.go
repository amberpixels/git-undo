package logging_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/amberpixels/git-undo/internal/git-undo/logging"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToggleLine(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	// Test cases
	tests := []struct {
		name        string
		initialData string
		lineNumber  int
		expected    string
	}{
		{
			name:        "Comment first line",
			initialData: "Line 1\nLine 2\nLine 3\n",
			lineNumber:  0,
			expected:    "#Line 1\nLine 2\nLine 3\n",
		},
		{
			name:        "Uncomment first line",
			initialData: "#Line 1\nLine 2\nLine 3\n",
			lineNumber:  0,
			expected:    "Line 1\nLine 2\nLine 3\n",
		},
		{
			name:        "Comment middle line",
			initialData: "Line 1\nLine 2\nLine 3\n",
			lineNumber:  1,
			expected:    "Line 1\n#Line 2\nLine 3\n",
		},
		{
			name:        "Uncomment middle line",
			initialData: "Line 1\n#Line 2\nLine 3\n",
			lineNumber:  1,
			expected:    "Line 1\nLine 2\nLine 3\n",
		},
		{
			name:        "Comment last line",
			initialData: "Line 1\nLine 2\nLine 3",
			lineNumber:  2,
			expected:    "Line 1\nLine 2\n#Line 3",
		},
		{
			name:        "Uncomment last line",
			initialData: "Line 1\nLine 2\n#Line 3",
			lineNumber:  2,
			expected:    "Line 1\nLine 2\nLine 3",
		},
		{
			name:        "Comment line with spaces",
			initialData: "Line 1\n  Line 2\nLine 3\n",
			lineNumber:  1,
			expected:    "Line 1\n#  Line 2\nLine 3\n",
		},
		{
			name:        "Uncomment line with spaces after comment",
			initialData: "Line 1\n#  Line 2\nLine 3\n",
			lineNumber:  1,
			expected:    "Line 1\n  Line 2\nLine 3\n",
		},
		{
			name:        "Single line file",
			initialData: "Line 1",
			lineNumber:  0,
			expected:    "#Line 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary test file
			testFile := filepath.Join(tmpDir, "test_toggle_line.txt")
			err := os.WriteFile(testFile, []byte(tt.initialData), 0644)
			require.NoError(t, err, "Failed to write test file")

			// Open the file for reading and writing
			file, err := os.OpenFile(testFile, os.O_RDWR, 0644)
			require.NoError(t, err, "Failed to open test file")
			defer file.Close()

			// Test the toggleLine function
			err = logging.ToggleLine(file, tt.lineNumber)
			assert.NoError(t, err, "toggleLine returned an error")

			// Read the file content after toggling
			file.Seek(0, 0) // Reset file pointer to beginning
			result, err := io.ReadAll(file)
			require.NoError(t, err, "Failed to read file content")

			// Compare the result with expected output
			assert.Equal(t, tt.expected, string(result), "File content does not match expected output")
		})
	}
}
