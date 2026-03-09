package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/meganerd/pi-go/internal/message"
	"github.com/meganerd/pi-go/internal/provider"
	"github.com/meganerd/pi-go/internal/tool"
)

// mockProvider simulates an LLM that calls a tool then responds with text.
type mockProvider struct {
	callCount int
	responses []*provider.ChatResponse
}

func (m *mockProvider) Name() string { return "mock" }
func (m *mockProvider) Models() []provider.Model {
	return []provider.Model{{ID: "mock-model"}}
}

func (m *mockProvider) Chat(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	idx := m.callCount
	m.callCount++
	if idx < len(m.responses) {
		return m.responses[idx], nil
	}
	// Default: text response to end loop
	return &provider.ChatResponse{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: "Done.",
		},
	}, nil
}

// mockTool returns a fixed result.
type mockTool struct {
	name   string
	result string
}

func (t *mockTool) Name() string                                       { return t.name }
func (t *mockTool) Description() string                                { return "mock tool" }
func (t *mockTool) Schema() json.RawMessage                            { return json.RawMessage(`{}`) }
func (t *mockTool) Execute(ctx context.Context, input json.RawMessage) (*tool.Result, error) {
	return &tool.Result{Output: t.result}, nil
}

func TestAgentLoop_MultiTurnToolCalling(t *testing.T) {
	// Simulate: user asks -> LLM calls "read" tool -> we return result -> LLM gives final text
	mp := &mockProvider{
		responses: []*provider.ChatResponse{
			{
				Message: message.Message{
					Role:    message.RoleAssistant,
					Content: "I'll read that file.",
					ToolCalls: []message.ToolCall{
						{
							ID:    "call_1",
							Name:  "read",
							Input: json.RawMessage(`{"path":"/tmp/test.txt"}`),
						},
					},
				},
			},
			{
				Message: message.Message{
					Role:    message.RoleAssistant,
					Content: "The file contains: hello world",
				},
			},
		},
	}

	tools := []tool.Tool{
		&mockTool{name: "read", result: "hello world"},
	}

	loop := New(mp, tools)
	resp, err := loop.Run(context.Background(), &provider.ChatRequest{
		Model: "mock-model",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: "Read /tmp/test.txt"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Provider should have been called twice: initial + after tool result
	if mp.callCount != 2 {
		t.Errorf("expected 2 provider calls, got %d", mp.callCount)
	}

	// Final response should be the text response
	if resp.Message.Content != "The file contains: hello world" {
		t.Errorf("expected final text, got: %q", resp.Message.Content)
	}

	// Final response should not have tool calls
	if resp.Message.HasToolCalls() {
		t.Error("expected no tool calls in final response")
	}
}

func TestAgentLoop_NoToolCalls(t *testing.T) {
	// Simple text response, no tools needed
	mp := &mockProvider{
		responses: []*provider.ChatResponse{
			{
				Message: message.Message{
					Role:    message.RoleAssistant,
					Content: "Hello! How can I help?",
				},
			},
		},
	}

	loop := New(mp, nil)
	resp, err := loop.Run(context.Background(), &provider.ChatRequest{
		Model: "mock-model",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: "Hi"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mp.callCount != 1 {
		t.Errorf("expected 1 provider call, got %d", mp.callCount)
	}
	if resp.Message.Content != "Hello! How can I help?" {
		t.Errorf("unexpected content: %q", resp.Message.Content)
	}
}
