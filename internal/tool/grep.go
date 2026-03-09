package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// GrepTool searches file contents using regex patterns via ripgrep.
type GrepTool struct{}

type grepInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
	Glob    string `json:"glob,omitempty"`
}

func (t *GrepTool) Name() string        { return "grep" }
func (t *GrepTool) Description() string { return "Search file contents using regex patterns" }

func (t *GrepTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {"type": "string", "description": "Regex pattern to search for"},
			"path": {"type": "string", "description": "File or directory to search in"},
			"glob": {"type": "string", "description": "Glob pattern to filter files"}
		},
		"required": ["pattern"]
	}`)
}

func (t *GrepTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var in grepInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	args := []string{"-n", "--no-heading", in.Pattern}
	if in.Glob != "" {
		args = append(args, "--glob", in.Glob)
	}
	if in.Path != "" {
		args = append(args, in.Path)
	} else {
		args = append(args, ".")
	}

	cmd := exec.CommandContext(ctx, "rg", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()

	// rg exits 1 for no matches — that's not an error
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return &Result{Output: "", Lines: 0}, nil
		}
		return nil, fmt.Errorf("rg failed: %s", stderr.String())
	}

	lines := strings.Count(output, "\n")
	return &Result{
		Output: output,
		Lines:  lines,
		Bytes:  len(output),
	}, nil
}
