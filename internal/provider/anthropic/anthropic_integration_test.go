//go:build integration

package anthropic

import (
	"context"
	"os"
	"testing"

	"github.com/meganerd/pi-go/internal/message"
	"github.com/meganerd/pi-go/internal/provider"
)

// Run with: ANTHROPIC_API_KEY=... go test -tags integration -run TestIntegration ./internal/provider/anthropic/

func TestIntegration_SimpleChat(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}

	p := New(apiKey, "")
	resp, err := p.Chat(context.Background(), &provider.ChatRequest{
		Model: "claude-haiku-3-5-20241022",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: "Reply with exactly the word 'pong'. Nothing else."},
		},
		MaxTokens: 32,
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if resp.Message.Role != message.RoleAssistant {
		t.Errorf("role = %q, want assistant", resp.Message.Role)
	}
	if resp.Message.Content == "" {
		t.Error("expected non-empty response content")
	}
	if resp.Usage.InputTokens == 0 {
		t.Error("expected non-zero input tokens")
	}
	if resp.Usage.OutputTokens == 0 {
		t.Error("expected non-zero output tokens")
	}
	if resp.StopReason == "" {
		t.Error("expected non-empty stop reason")
	}
	t.Logf("Response: %q (stop: %s, %d in / %d out tokens)",
		resp.Message.Content, resp.StopReason, resp.Usage.InputTokens, resp.Usage.OutputTokens)
}

func TestIntegration_ToolCall(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}

	p := New(apiKey, "")
	resp, err := p.Chat(context.Background(), &provider.ChatRequest{
		Model: "claude-haiku-3-5-20241022",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: "Read the file /tmp/test.txt"},
		},
		Tools: []provider.ToolDef{
			{
				Name:        "read",
				Description: "Read a file's contents",
				Schema:      []byte(`{"type":"object","properties":{"path":{"type":"string","description":"File path"}},"required":["path"]}`),
			},
		},
		MaxTokens: 256,
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if !resp.Message.HasToolCalls() {
		t.Fatal("expected tool calls in response")
	}

	tc := resp.Message.ToolCalls[0]
	if tc.Name != "read" {
		t.Errorf("tool name = %q, want read", tc.Name)
	}
	if tc.ID == "" {
		t.Error("expected non-empty tool call ID")
	}
	t.Logf("Tool call: %s(id=%s, input=%s)", tc.Name, tc.ID, string(tc.Input))
}

func TestIntegration_Streaming(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}

	p := New(apiKey, "")
	ch, err := p.StreamChat(context.Background(), &provider.ChatRequest{
		Model: "claude-haiku-3-5-20241022",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: "Say hello in exactly 3 words."},
		},
		MaxTokens: 32,
	})
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}

	var textEvents, doneEvents int
	var fullText string
	for event := range ch {
		switch event.Type {
		case "text":
			textEvents++
			fullText += event.Content
		case "done":
			doneEvents++
		case "error":
			t.Fatalf("stream error: %v", event.Error)
		}
	}

	if textEvents == 0 {
		t.Error("expected at least one text event")
	}
	if doneEvents != 1 {
		t.Errorf("expected exactly 1 done event, got %d", doneEvents)
	}
	if fullText == "" {
		t.Error("expected non-empty accumulated text")
	}
	t.Logf("Streamed %d text events: %q", textEvents, fullText)
}
