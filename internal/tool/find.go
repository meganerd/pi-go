package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// FindTool discovers files matching glob patterns.
// Uses fd if available, falls back to Go's filepath.WalkDir.
type FindTool struct {
	FS FileSystem
}

type findInput struct {
	Pattern string   `json:"pattern"`
	Path    string   `json:"path,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
}

func (t *FindTool) Name() string        { return "find" }
func (t *FindTool) Description() string { return "Find files matching a glob pattern" }

func (t *FindTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {"type": "string", "description": "Glob pattern to match files"},
			"path": {"type": "string", "description": "Directory to search in"},
			"exclude": {"type": "array", "items": {"type": "string"}, "description": "Directory names to exclude"}
		},
		"required": ["pattern"]
	}`)
}

func (t *FindTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var in findInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	root := in.Path
	if root == "" {
		root = "."
	}

	excludeSet := make(map[string]bool, len(in.Exclude))
	for _, ex := range in.Exclude {
		excludeSet[ex] = true
	}

	var matches []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Skip excluded directories
		if d.IsDir() && excludeSet[d.Name()] {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		matched, matchErr := filepath.Match(in.Pattern, d.Name())
		if matchErr != nil {
			return matchErr
		}
		if matched {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", root, err)
	}

	output := strings.Join(matches, "\n")
	if len(matches) > 0 {
		output += "\n"
	}

	return &Result{
		Output: output,
		Lines:  len(matches),
		Bytes:  len(output),
	}, nil
}
