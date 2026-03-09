package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
)

// WriteTool creates or overwrites files with automatic directory creation.
type WriteTool struct {
	FS FileSystem
}

type writeInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (t *WriteTool) Name() string        { return "write" }
func (t *WriteTool) Description() string { return "Write content to a file, creating directories as needed" }

func (t *WriteTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Absolute path to write to"},
			"content": {"type": "string", "description": "Content to write"}
		},
		"required": ["path", "content"]
	}`)
}

func (t *WriteTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var in writeInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Ensure parent directory exists
	dir := filepath.Dir(in.Path)
	if err := t.FS.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dir, err)
	}

	if err := t.FS.WriteFile(in.Path, []byte(in.Content), 0644); err != nil {
		return nil, fmt.Errorf("write %s: %w", in.Path, err)
	}

	return &Result{
		Output: fmt.Sprintf("Wrote %d bytes to %s", len(in.Content), in.Path),
		Bytes:  len(in.Content),
	}, nil
}
