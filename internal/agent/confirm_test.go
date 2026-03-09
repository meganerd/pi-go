package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/meganerd/pi-go/internal/message"
	"github.com/meganerd/pi-go/internal/provider"
	"github.com/meganerd/pi-go/internal/tool"
)

func TestIsReadOnly(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"read", true},
		{"grep", true},
		{"find", true},
		{"ls", true},
		{"bash", false},
		{"write", false},
		{"edit", false},
		{"et_delegate", false},
		{"unknown", false},
	}
	for _, tt := range tests {
		if got := IsReadOnly(tt.name); got != tt.want {
			t.Errorf("IsReadOnly(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// echoTool is a simple tool that echoes its input for testing.
type echoTool struct{ name string }

func (e *echoTool) Name() string                        { return e.name }
func (e *echoTool) Description() string                 { return "echo tool" }
func (e *echoTool) Schema() json.RawMessage              { return json.RawMessage(`{"type":"object"}`) }
func (e *echoTool) Execute(_ context.Context, input json.RawMessage) (*tool.Result, error) {
	return &tool.Result{Output: "executed: " + string(input)}, nil
}

// confirmMockProvider returns a response with a tool call on first call, text on second.
type confirmMockProvider struct {
	calls int
}

func (m *confirmMockProvider) Name() string            { return "mock" }
func (m *confirmMockProvider) Models() []provider.Model { return nil }
func (m *confirmMockProvider) Chat(_ context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	m.calls++
	if m.calls == 1 {
		return &provider.ChatResponse{
			Message: message.Message{
				Role: message.RoleAssistant,
				ToolCalls: []message.ToolCall{
					{ID: "tc1", Name: "bash", Input: json.RawMessage(`{"command":"ls"}`)},
				},
			},
		}, nil
	}
	return &provider.ChatResponse{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: "Done",
		},
	}, nil
}

func TestConfirmCallback_Approved(t *testing.T) {
	prov := &confirmMockProvider{}
	loop := New(prov, []tool.Tool{&echoTool{name: "bash"}}).
		WithConfirmCallback(func(name string, _ json.RawMessage) bool {
			return true // approve everything
		})

	resp, err := loop.Run(context.Background(), &provider.ChatRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message.Content != "Done" {
		t.Errorf("expected Done, got %q", resp.Message.Content)
	}
	if prov.calls != 2 {
		t.Errorf("expected 2 provider calls (tool + response), got %d", prov.calls)
	}
}

func TestConfirmCallback_Declined(t *testing.T) {
	prov := &confirmMockProvider{}
	loop := New(prov, []tool.Tool{&echoTool{name: "bash"}}).
		WithConfirmCallback(func(name string, _ json.RawMessage) bool {
			return false // decline everything
		})

	resp, err := loop.Run(context.Background(), &provider.ChatRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// After declining, the loop should send the declined result back to the LLM
	// and the LLM responds with text on the second call
	if prov.calls != 2 {
		t.Errorf("expected 2 provider calls, got %d", prov.calls)
	}
	// The declined message should have been sent to the provider
	if resp.Message.Content != "Done" {
		t.Errorf("expected Done, got %q", resp.Message.Content)
	}
}

// readOnlyMockProvider returns a response with a read tool call.
type readOnlyMockProvider struct {
	calls int
}

func (m *readOnlyMockProvider) Name() string            { return "mock" }
func (m *readOnlyMockProvider) Models() []provider.Model { return nil }
func (m *readOnlyMockProvider) Chat(_ context.Context, _ *provider.ChatRequest) (*provider.ChatResponse, error) {
	m.calls++
	if m.calls == 1 {
		return &provider.ChatResponse{
			Message: message.Message{
				Role: message.RoleAssistant,
				ToolCalls: []message.ToolCall{
					{ID: "tc1", Name: "read", Input: json.RawMessage(`{"path":"/tmp"}`)},
				},
			},
		}, nil
	}
	return &provider.ChatResponse{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: "Done",
		},
	}, nil
}

func TestConfirmCallback_ReadOnlySkipsConfirmation(t *testing.T) {
	callbackCalled := false
	prov := &readOnlyMockProvider{}
	loop := New(prov, []tool.Tool{&echoTool{name: "read"}}).
		WithConfirmCallback(func(name string, _ json.RawMessage) bool {
			callbackCalled = true
			return false // would decline if called
		})

	resp, err := loop.Run(context.Background(), &provider.ChatRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callbackCalled {
		t.Error("confirm callback should not be called for read-only tools")
	}
	if resp.Message.Content != "Done" {
		t.Errorf("expected Done, got %q", resp.Message.Content)
	}
}

func TestConfirmCallback_NilCallbackAutoApproves(t *testing.T) {
	prov := &confirmMockProvider{}
	loop := New(prov, []tool.Tool{&echoTool{name: "bash"}})
	// No WithConfirmCallback — should auto-approve

	resp, err := loop.Run(context.Background(), &provider.ChatRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message.Content != "Done" {
		t.Errorf("expected Done, got %q", resp.Message.Content)
	}
}

func TestConfirmCallback_DeclinedMessageContent(t *testing.T) {
	// Verify the declined message contains the right text
	prov := &confirmMockProvider{}
	var toolResults []string
	loop := New(prov, []tool.Tool{&echoTool{name: "bash"}}).
		WithConfirmCallback(func(name string, _ json.RawMessage) bool {
			return false
		}).
		WithToolCallback(func(name string, isResult bool, output string, isError bool) {
			if isResult && isError {
				toolResults = append(toolResults, output)
			}
		})

	_, err := loop.Run(context.Background(), &provider.ChatRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(toolResults) != 1 {
		t.Fatalf("expected 1 declined tool result, got %d", len(toolResults))
	}
	if !strings.Contains(toolResults[0], "declined") {
		t.Errorf("expected declined message, got %q", toolResults[0])
	}
}
