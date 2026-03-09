package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// LsTool lists directory contents.
type LsTool struct {
	FS FileSystem
}

type lsInput struct {
	Path  string `json:"path"`
	Limit int    `json:"limit,omitempty"`
}

const defaultLsLimit = 200

func (t *LsTool) Name() string        { return "ls" }
func (t *LsTool) Description() string { return "List directory contents" }

func (t *LsTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Directory path to list"},
			"limit": {"type": "integer", "description": "Maximum entries to show"}
		},
		"required": ["path"]
	}`)
}

func (t *LsTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var in lsInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	entries, err := t.FS.ReadDir(in.Path)
	if err != nil {
		return nil, fmt.Errorf("readdir %s: %w", in.Path, err)
	}

	limit := defaultLsLimit
	if in.Limit > 0 {
		limit = in.Limit
	}

	truncated := false
	if len(entries) > limit {
		entries = entries[:limit]
		truncated = true
	}

	var sb strings.Builder
	for _, e := range entries {
		suffix := ""
		if e.IsDir {
			suffix = "/"
		}
		fmt.Fprintf(&sb, "%s%s\n", e.Name, suffix)
	}

	if truncated {
		fmt.Fprintf(&sb, "\n[truncated — showing %d of %d entries]", limit, len(entries)+1)
	}

	return &Result{
		Output:    sb.String(),
		Truncated: truncated,
		Lines:     len(entries),
	}, nil
}
