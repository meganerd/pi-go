package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// BashTool executes shell commands with timeout support.
type BashTool struct{}

type bashInput struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"` // milliseconds
}

func (t *BashTool) Name() string        { return "bash" }
func (t *BashTool) Description() string { return "Execute a bash command" }

func (t *BashTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "The bash command to execute"},
			"timeout": {"type": "integer", "description": "Timeout in milliseconds (default 600000)"}
		},
		"required": ["command"]
	}`)
}

func (t *BashTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var in bashInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	timeout := 600 * time.Second // default 10 minutes
	if in.Timeout > 0 {
		timeout = time.Duration(in.Timeout) * time.Millisecond
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", in.Command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	if ctx.Err() == context.DeadlineExceeded {
		return &Result{
			Output:    output + "\n[command timed out]",
			Truncated: true,
		}, nil
	}

	if err != nil {
		// Command failed but produced output — return output with error info
		exitErr, ok := err.(*exec.ExitError)
		if ok {
			output += fmt.Sprintf("\n[exit code: %d]", exitErr.ExitCode())
		}
		return &Result{Output: output}, nil
	}

	return &Result{
		Output: output,
		Bytes:  len(output),
	}, nil
}
