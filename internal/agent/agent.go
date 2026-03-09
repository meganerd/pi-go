// Package agent implements the tool-calling agent loop for pi-go.
package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/meganerd/pi-go/internal/message"
	"github.com/meganerd/pi-go/internal/provider"
	"github.com/meganerd/pi-go/internal/tool"
)

// Loop orchestrates the conversation between the LLM and tools.
type Loop struct {
	provider provider.Provider
	tools    map[string]tool.Tool
}

// New creates a new agent loop with the given provider and tools.
func New(p provider.Provider, tools []tool.Tool) *Loop {
	toolMap := make(map[string]tool.Tool)
	for _, t := range tools {
		toolMap[t.Name()] = t
	}
	return &Loop{
		provider: p,
		tools:    toolMap,
	}
}

// Run executes the agent loop: send to LLM, execute tool calls, repeat until text-only response.
func (l *Loop) Run(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	// Add tool definitions to request
	if len(l.tools) > 0 && len(req.Tools) == 0 {
		for _, t := range l.tools {
			req.Tools = append(req.Tools, provider.ToolDef{
				Name:        t.Name(),
				Description: t.Description(),
				Schema:      t.Schema(),
			})
		}
	}

	const maxIterations = 20 // safety limit
	for i := 0; i < maxIterations; i++ {
		resp, err := l.provider.Chat(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("provider chat (iteration %d): %w", i, err)
		}

		// If no tool calls, we're done
		if !resp.Message.HasToolCalls() {
			return resp, nil
		}

		// Append assistant message with tool calls to history
		req.Messages = append(req.Messages, resp.Message)

		// Execute each tool call and append results
		for _, tc := range resp.Message.ToolCalls {
			result, err := l.executeTool(ctx, tc)
			if err != nil {
				// Send error back to LLM
				req.Messages = append(req.Messages, message.Message{
					Role: message.RoleTool,
					ToolResult: &message.ToolResultMsg{
						ToolCallID: tc.ID,
						Content:    fmt.Sprintf("Error: %v", err),
						IsError:    true,
					},
				})
				continue
			}

			req.Messages = append(req.Messages, message.Message{
				Role: message.RoleTool,
				ToolResult: &message.ToolResultMsg{
					ToolCallID: tc.ID,
					Content:    result.Output,
				},
			})
		}
	}

	return nil, fmt.Errorf("agent loop exceeded %d iterations", maxIterations)
}

func (l *Loop) executeTool(ctx context.Context, tc message.ToolCall) (*tool.Result, error) {
	t, ok := l.tools[tc.Name]
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", tc.Name)
	}

	input := tc.Input
	if input == nil {
		input = json.RawMessage(`{}`)
	}

	return t.Execute(ctx, input)
}
