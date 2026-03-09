package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ReadTool reads file contents with optional offset and limit.
type ReadTool struct {
	FS FileSystem
}

type readInput struct {
	Path   string `json:"path"`
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

func (t *ReadTool) Name() string        { return "read" }
func (t *ReadTool) Description() string { return "Read a file's contents" }

func (t *ReadTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Absolute path to the file to read"},
			"offset": {"type": "integer", "description": "Line number to start reading from (1-based)"},
			"limit": {"type": "integer", "description": "Maximum number of lines to read"}
		},
		"required": ["path"]
	}`)
}

func (t *ReadTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var in readInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	data, err := t.FS.ReadFile(in.Path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", in.Path, err)
	}

	lines := strings.Split(string(data), "\n")
	totalLines := len(lines)

	// Apply offset (1-based)
	offset := 0
	if in.Offset > 0 {
		offset = in.Offset - 1
		if offset >= len(lines) {
			return &Result{Output: "", Lines: totalLines, Bytes: len(data)}, nil
		}
		lines = lines[offset:]
	}

	// Apply limit
	truncated := false
	if in.Limit > 0 && in.Limit < len(lines) {
		lines = lines[:in.Limit]
		truncated = true
	}

	// Format with line numbers
	var sb strings.Builder
	for i, line := range lines {
		lineNum := offset + i + 1
		fmt.Fprintf(&sb, "%6d\t%s\n", lineNum, line)
	}

	return &Result{
		Output:    sb.String(),
		Truncated: truncated,
		Lines:     totalLines,
		Bytes:     len(data),
	}, nil
}
