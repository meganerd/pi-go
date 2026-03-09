package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/meganerd/pi-go/internal/message"
	"github.com/meganerd/pi-go/internal/provider"
	"github.com/meganerd/pi-go/internal/session"
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

func TestAgentLoop_SessionPersistence(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "session.jsonl")

	store, err := session.NewJSONLStore(storePath)
	if err != nil {
		t.Fatal(err)
	}

	// Persist the user message before running
	userMsg := message.Message{Role: message.RoleUser, Content: "What is 2+2?"}
	store.Append(&userMsg)

	mp := &mockProvider{
		responses: []*provider.ChatResponse{
			{
				Message: message.Message{
					Role:    message.RoleAssistant,
					Content: "2+2 is 4.",
				},
			},
		},
	}

	loop := New(mp, nil).WithSession(store)
	resp, err := loop.Run(context.Background(), &provider.ChatRequest{
		Model: "mock-model",
		Messages: []message.Message{
			userMsg,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Message.Content != "2+2 is 4." {
		t.Errorf("unexpected: %q", resp.Message.Content)
	}
	store.Close()

	// Reopen and verify messages survived
	store2, err := session.NewJSONLStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store2.Close()

	history, err := store2.Messages()
	if err != nil {
		t.Fatal(err)
	}
	// user + assistant = 2 messages
	if len(history) != 2 {
		t.Fatalf("expected 2 persisted messages, got %d", len(history))
	}
	if history[0].Role != message.RoleUser {
		t.Errorf("msg[0] role: got %q, want %q", history[0].Role, message.RoleUser)
	}
	if history[1].Role != message.RoleAssistant {
		t.Errorf("msg[1] role: got %q, want %q", history[1].Role, message.RoleAssistant)
	}
	if history[1].Content != "2+2 is 4." {
		t.Errorf("msg[1] content: got %q, want %q", history[1].Content, "2+2 is 4.")
	}
}

func TestAgentLoop_Resume(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "session.jsonl")

	// First session: write two messages
	store1, _ := session.NewJSONLStore(storePath)
	msg1 := message.Message{Role: message.RoleUser, Content: "Hello"}
	store1.Append(&msg1)
	msg2 := message.Message{Role: message.RoleAssistant, Content: "Hi there!"}
	store1.Append(&msg2)
	store1.Close()

	// Reopen and resume
	store2, _ := session.NewJSONLStore(storePath)

	mp := &mockProvider{
		responses: []*provider.ChatResponse{
			{
				Message: message.Message{
					Role:    message.RoleAssistant,
					Content: "Resumed successfully.",
				},
			},
		},
	}

	loop := New(mp, nil).WithSession(store2)
	req := &provider.ChatRequest{Model: "mock-model"}

	// Resume loads persisted messages
	if err := loop.Resume(req); err != nil {
		t.Fatal(err)
	}
	if len(req.Messages) != 2 {
		t.Fatalf("expected 2 resumed messages, got %d", len(req.Messages))
	}

	// Add new user message and run
	newMsg := message.Message{Role: message.RoleUser, Content: "Continue"}
	store2.Append(&newMsg)
	req.Messages = append(req.Messages, newMsg)

	resp, err := loop.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Message.Content != "Resumed successfully." {
		t.Errorf("unexpected: %q", resp.Message.Content)
	}
	store2.Close()

	// Verify full chain persisted
	store3, _ := session.NewJSONLStore(storePath)
	defer store3.Close()
	history, _ := store3.Messages()
	// Hello + Hi there! + Continue + Resumed successfully. = 4
	if len(history) != 4 {
		t.Fatalf("expected 4 messages after resume, got %d", len(history))
	}
}

// mockStreamProvider simulates a streaming LLM.
type mockStreamProvider struct {
	mockProvider
	streamCallCount int
	eventSets       [][]provider.StreamEvent // one set per StreamChat call
}

func (m *mockStreamProvider) StreamChat(ctx context.Context, req *provider.ChatRequest) (<-chan provider.StreamEvent, error) {
	idx := m.streamCallCount
	m.streamCallCount++
	if idx >= len(m.eventSets) {
		// No more stream events — fall back to non-streaming
		return nil, fmt.Errorf("no more stream events")
	}
	events := m.eventSets[idx]
	ch := make(chan provider.StreamEvent, len(events))
	for _, e := range events {
		ch <- e
	}
	close(ch)
	return ch, nil
}

func TestAgentLoop_Streaming(t *testing.T) {
	sp := &mockStreamProvider{
		eventSets: [][]provider.StreamEvent{
			{
				{Type: "text", Content: "Hello "},
				{Type: "text", Content: "World"},
				{Type: "done"},
			},
		},
	}

	var streamed string
	loop := New(sp, nil).WithStreamCallback(func(text string) {
		streamed += text
	})

	resp, err := loop.Run(context.Background(), &provider.ChatRequest{
		Model:    "mock-model",
		Messages: []message.Message{{Role: message.RoleUser, Content: "Hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if resp.Message.Content != "Hello World" {
		t.Errorf("response content = %q, want 'Hello World'", resp.Message.Content)
	}
	if streamed != "Hello World" {
		t.Errorf("streamed = %q, want 'Hello World'", streamed)
	}
}

func TestAgentLoop_StreamingWithToolCall(t *testing.T) {
	sp := &mockStreamProvider{
		eventSets: [][]provider.StreamEvent{
			{
				{Type: "text", Content: "Let me read that."},
				{Type: "tool_use_start", ToolID: "call_1", ToolName: "read"},
				{Type: "tool_use_delta", Content: `{"path":"/tmp`},
				{Type: "tool_use_delta", Content: `/test.txt"}`},
				{Type: "tool_use_end"},
				{Type: "done"},
			},
			// Second stream call returns error, forcing fallback to Chat()
		},
	}
	// After tool execution, Chat fallback returns the final text response
	sp.responses = []*provider.ChatResponse{
		{Message: message.Message{Role: message.RoleAssistant, Content: "File says: hello"}},
	}

	tools := []tool.Tool{
		&mockTool{name: "read", result: "hello"},
	}

	loop := New(sp, tools).WithStreamCallback(func(text string) {})

	resp, err := loop.Run(context.Background(), &provider.ChatRequest{
		Model:    "mock-model",
		Messages: []message.Message{{Role: message.RoleUser, Content: "Read test.txt"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if resp.Message.Content != "File says: hello" {
		t.Errorf("final content = %q", resp.Message.Content)
	}
}
