package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/meganerd/pi-go/internal/et"
)

// EtDelegateTool delegates code generation tasks to electrictown workers.
type EtDelegateTool struct {
	Delegator et.Delegator
}

type etInput struct {
	Task      string `json:"task"`
	OutputDir string `json:"output_dir,omitempty"`
	Iterate   bool   `json:"iterate,omitempty"`
}

func (t *EtDelegateTool) Name() string { return "et_delegate" }

func (t *EtDelegateTool) Description() string {
	return "Delegate a code generation task to electrictown (et) workers for parallel execution using local LLM models. Use for file generation, test writing, refactoring, or any task that benefits from parallel worker execution."
}

func (t *EtDelegateTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"task": {
				"type": "string",
				"description": "The task description for et workers. Be specific about what code to generate, which files to create, and any requirements."
			},
			"output_dir": {
				"type": "string",
				"description": "Directory where et should write output files. Optional — defaults to /tmp/pi-go-et."
			},
			"iterate": {
				"type": "boolean",
				"description": "Enable iterative build/fix loop. Workers will compile and fix errors automatically."
			}
		},
		"required": ["task"]
	}`)
}

func (t *EtDelegateTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	if t.Delegator == nil || !t.Delegator.Available() {
		return &Result{
			Output: "Error: electrictown (et) is not available. Ensure 'et' is installed and ~/electrictown.yaml exists.",
		}, nil
	}

	var inp etInput
	if err := json.Unmarshal(input, &inp); err != nil {
		return nil, fmt.Errorf("parse et_delegate input: %w", err)
	}

	if inp.Task == "" {
		return &Result{Output: "Error: task is required"}, nil
	}

	opts := &et.Options{
		OutputDir: inp.OutputDir,
		Iterate:   inp.Iterate,
	}

	result, err := t.Delegator.Delegate(ctx, inp.Task, opts)
	if err != nil {
		return &Result{Output: fmt.Sprintf("et delegation error: %v", err)}, nil
	}

	var sb strings.Builder
	if result.ExitCode != 0 {
		fmt.Fprintf(&sb, "et exited with code %d\n\n", result.ExitCode)
	}
	sb.WriteString(result.Output)
	if len(result.Files) > 0 {
		sb.WriteString("\n\nOutput files:\n")
		for _, f := range result.Files {
			fmt.Fprintf(&sb, "  %s\n", f)
		}
	}

	return &Result{Output: sb.String()}, nil
}
