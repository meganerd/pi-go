package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// EditTool performs find-and-replace operations on files.
type EditTool struct {
	FS FileSystem
}

type editInput struct {
	Path      string `json:"path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

func (t *EditTool) Name() string        { return "edit" }
func (t *EditTool) Description() string { return "Find and replace text in a file" }

func (t *EditTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Absolute path to the file"},
			"old_string": {"type": "string", "description": "Text to find"},
			"new_string": {"type": "string", "description": "Replacement text"}
		},
		"required": ["path", "old_string", "new_string"]
	}`)
}

func (t *EditTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var in editInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	data, err := t.FS.ReadFile(in.Path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", in.Path, err)
	}

	content := string(data)

	// Check uniqueness
	count := strings.Count(content, in.OldString)
	if count == 0 {
		return nil, fmt.Errorf("old_string not found in %s", in.Path)
	}
	if count > 1 {
		return nil, fmt.Errorf("old_string matches %d locations in %s — must be unique", count, in.Path)
	}

	// Perform replacement
	newContent := strings.Replace(content, in.OldString, in.NewString, 1)
	if err := t.FS.WriteFile(in.Path, []byte(newContent), 0644); err != nil {
		return nil, fmt.Errorf("write %s: %w", in.Path, err)
	}

	return &Result{
		Output: fmt.Sprintf("Replaced text in %s", in.Path),
		Bytes:  len(newContent),
	}, nil
}
