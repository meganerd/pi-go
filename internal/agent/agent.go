// Package agent implements the tool-calling agent loop for pi-go.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/meganerd/pi-go/internal/compact"
	"github.com/meganerd/pi-go/internal/message"
	"github.com/meganerd/pi-go/internal/provider"
	"github.com/meganerd/pi-go/internal/session"
	"github.com/meganerd/pi-go/internal/tool"
)

// ToolCallback is called when a tool is invoked or returns a result.
type ToolCallback func(toolName string, isResult bool, output string, isError bool)

// StreamCallback is called for each streaming text token.
type StreamCallback func(text string)

// Loop orchestrates the conversation between the LLM and tools.
type Loop struct {
	provider    provider.Provider
	tools       map[string]tool.Tool
	session     session.Store
	compactor   *compact.Compactor
	retryConfig provider.RetryConfig
	onToolCall  ToolCallback
	onStream    StreamCallback
	onConfirm   ConfirmCallback
	parallel    bool // when true, execute read-only tools in parallel
}

// New creates a new agent loop with the given provider and tools.
func New(p provider.Provider, tools []tool.Tool) *Loop {
	toolMap := make(map[string]tool.Tool)
	for _, t := range tools {
		toolMap[t.Name()] = t
	}
	return &Loop{
		provider:    p,
		tools:       toolMap,
		retryConfig: provider.DefaultRetryConfig(),
	}
}

// WithSession sets a session store for persisting messages.
func (l *Loop) WithSession(s session.Store) *Loop {
	l.session = s
	return l
}

// WithCompactor sets a context compactor for long conversations.
func (l *Loop) WithCompactor(c *compact.Compactor) *Loop {
	l.compactor = c
	return l
}

// WithToolCallback sets a callback for tool invocations and results.
func (l *Loop) WithToolCallback(cb ToolCallback) *Loop {
	l.onToolCall = cb
	return l
}

// WithStreamCallback sets a callback for streaming text tokens.
func (l *Loop) WithStreamCallback(cb StreamCallback) *Loop {
	l.onStream = cb
	return l
}

// WithParallelTools enables parallel execution of read-only tools.
func (l *Loop) WithParallelTools() *Loop {
	l.parallel = true
	return l
}

// Provider returns the agent's provider.
func (l *Loop) Provider() provider.Provider {
	return l.provider
}

// Compactor returns the agent's compactor, or nil if none is set.
func (l *Loop) Compactor() *compact.Compactor {
	return l.compactor
}

// WithConfirmCallback sets a callback for confirming tool execution.
// Read-only tools (read, grep, find, ls) are auto-approved.
func (l *Loop) WithConfirmCallback(cb ConfirmCallback) *Loop {
	l.onConfirm = cb
	return l
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
		// Compact context if needed
		if l.compactor != nil {
			compacted, err := l.compactor.Compact(ctx, req.Messages)
			if err == nil {
				req.Messages = compacted
			}
		}

		// Try streaming if available, fall back to non-streaming
		resp, err := l.chat(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("provider chat (iteration %d): %w", i, err)
		}

		// If no tool calls, we're done
		if !resp.Message.HasToolCalls() {
			l.persist(&resp.Message)
			return resp, nil
		}

		// Append assistant message with tool calls to history
		l.persist(&resp.Message)
		req.Messages = append(req.Messages, resp.Message)

		// Execute tool calls — parallel for read-only tools when enabled.
		toolMsgs := l.executeToolCalls(ctx, resp.Message.ToolCalls)
		for _, toolMsg := range toolMsgs {
			l.persist(&toolMsg)
			req.Messages = append(req.Messages, toolMsg)
		}
	}

	return nil, fmt.Errorf("agent loop exceeded %d iterations", maxIterations)
}

// chat attempts streaming if available, otherwise falls back to non-streaming with retry.
func (l *Loop) chat(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	sp, ok := l.provider.(provider.StreamProvider)
	if !ok || l.onStream == nil {
		return provider.RetryChat(ctx, l.provider, req, l.retryConfig)
	}

	ch, err := sp.StreamChat(ctx, req)
	if err != nil {
		// Fall back to non-streaming with retry on error
		return provider.RetryChat(ctx, l.provider, req, l.retryConfig)
	}

	return l.consumeStream(ch)
}

// consumeStream reads all events from a stream and assembles a ChatResponse.
func (l *Loop) consumeStream(ch <-chan provider.StreamEvent) (*provider.ChatResponse, error) {
	var textBuilder strings.Builder
	var toolCalls []message.ToolCall
	var currentTool *message.ToolCall
	var toolInputBuilder strings.Builder

	for event := range ch {
		switch event.Type {
		case "text":
			textBuilder.WriteString(event.Content)
			if l.onStream != nil {
				l.onStream(event.Content)
			}

		case "tool_use_start":
			// Finish previous tool if any
			if currentTool != nil {
				currentTool.Input = json.RawMessage(toolInputBuilder.String())
				toolCalls = append(toolCalls, *currentTool)
				toolInputBuilder.Reset()
			}
			currentTool = &message.ToolCall{
				ID:   event.ToolID,
				Name: event.ToolName,
			}

		case "tool_use_delta":
			toolInputBuilder.WriteString(event.Content)

		case "tool_use_end":
			if currentTool != nil {
				input := toolInputBuilder.String()
				if input == "" {
					input = "{}"
				}
				currentTool.Input = json.RawMessage(input)
				toolCalls = append(toolCalls, *currentTool)
				currentTool = nil
				toolInputBuilder.Reset()
			}

		case "done":
			// Finish any pending tool
			if currentTool != nil {
				input := toolInputBuilder.String()
				if input == "" {
					input = "{}"
				}
				currentTool.Input = json.RawMessage(input)
				toolCalls = append(toolCalls, *currentTool)
			}

		case "error":
			if event.Error != nil {
				return nil, event.Error
			}
			return nil, fmt.Errorf("stream error: %s", event.Content)
		}
	}

	msg := message.Message{
		Role:      message.RoleAssistant,
		Content:   textBuilder.String(),
		ToolCalls: toolCalls,
	}

	return &provider.ChatResponse{Message: msg}, nil
}

// Resume loads persisted messages from the session store into a ChatRequest.
func (l *Loop) Resume(req *provider.ChatRequest) error {
	if l.session == nil {
		return nil
	}
	msgs, err := l.session.Messages()
	if err != nil {
		return fmt.Errorf("resume session: %w", err)
	}
	req.Messages = append(req.Messages, msgs...)
	return nil
}

func (l *Loop) persist(msg *message.Message) {
	if l.session != nil {
		if err := l.session.Append(msg); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to persist message: %v\n", err)
		}
	}
}

// executeToolCalls runs tool calls, optionally in parallel for read-only tools.
// Results are always returned in the original order.
func (l *Loop) executeToolCalls(ctx context.Context, calls []message.ToolCall) []message.Message {
	// Check if all calls are read-only and parallel is enabled
	allReadOnly := l.parallel
	if allReadOnly {
		for _, tc := range calls {
			if !IsReadOnly(tc.Name) {
				allReadOnly = false
				break
			}
		}
	}

	if allReadOnly && len(calls) > 1 {
		return l.executeToolCallsParallel(ctx, calls)
	}
	return l.executeToolCallsSequential(ctx, calls)
}

func (l *Loop) executeToolCallsSequential(ctx context.Context, calls []message.ToolCall) []message.Message {
	var msgs []message.Message
	for _, tc := range calls {
		msgs = append(msgs, l.executeSingleToolCall(ctx, tc))
	}
	return msgs
}

func (l *Loop) executeToolCallsParallel(ctx context.Context, calls []message.ToolCall) []message.Message {
	results := make([]message.Message, len(calls))
	var wg sync.WaitGroup

	for i, tc := range calls {
		wg.Add(1)
		go func(idx int, call message.ToolCall) {
			defer wg.Done()
			results[idx] = l.executeSingleToolCall(ctx, call)
		}(i, tc)
	}

	wg.Wait()
	return results
}

func (l *Loop) executeSingleToolCall(ctx context.Context, tc message.ToolCall) message.Message {
	if l.onToolCall != nil {
		l.onToolCall(tc.Name, false, "", false)
	}

	// Check confirmation for non-read-only tools
	if l.onConfirm != nil && !IsReadOnly(tc.Name) {
		if !l.onConfirm(tc.Name, tc.Input) {
			declinedMsg := "Tool execution declined by user"
			if l.onToolCall != nil {
				l.onToolCall(tc.Name, true, declinedMsg, true)
			}
			return message.Message{
				Role: message.RoleTool,
				ToolResult: &message.ToolResultMsg{
					ToolCallID: tc.ID,
					Content:    declinedMsg,
					IsError:    true,
				},
			}
		}
	}

	result, err := l.executeTool(ctx, tc)
	if err != nil {
		errContent := fmt.Sprintf("Error: %v", err)
		if l.onToolCall != nil {
			l.onToolCall(tc.Name, true, errContent, true)
		}
		return message.Message{
			Role: message.RoleTool,
			ToolResult: &message.ToolResultMsg{
				ToolCallID: tc.ID,
				Content:    errContent,
				IsError:    true,
			},
		}
	}

	if l.onToolCall != nil {
		l.onToolCall(tc.Name, true, result.Output, false)
	}
	return message.Message{
		Role: message.RoleTool,
		ToolResult: &message.ToolResultMsg{
			ToolCallID: tc.ID,
			Content:    result.Output,
		},
	}
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
