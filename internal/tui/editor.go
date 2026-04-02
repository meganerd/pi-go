package tui

import (
	"os"
	"os/exec"
	"strings"
)

// openExternalEditor launches $VISUAL or $EDITOR with a temp file,
// waits for the user to save and close, then returns the file content.
// Returns empty string if the editor is not configured or the file is empty.
func openExternalEditor() (string, error) {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}

	tmpFile, err := os.CreateTemp("", "pi-go-edit-*.md")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer os.Remove(tmpPath)

	cmd := exec.Command(editor, tmpPath) //nolint:gosec // User-configured editor
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", err
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}
