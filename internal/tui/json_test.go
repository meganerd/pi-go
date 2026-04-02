package tui

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/meganerd/pi-go/internal/message"
	"github.com/meganerd/pi-go/internal/provider"
)

func TestJSONWriter_EmitMessage(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONWriter(&buf)

	jw.EmitMessage(message.Message{
		Role:    message.RoleAssistant,
		Content: "Hello!",
	})

	var event JSONEvent
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if event.Type != "message" {
		t.Errorf("expected type 'message', got %q", event.Type)
	}
	if event.Role != "assistant" {
		t.Errorf("expected role 'assistant', got %q", event.Role)
	}
	if event.Content != "Hello!" {
		t.Errorf("expected content 'Hello!', got %q", event.Content)
	}
}

func TestJSONWriter_EmitToolCall(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONWriter(&buf)

	jw.EmitToolCall("read", "tc1", json.RawMessage(`{"path":"/tmp"}`))

	var event JSONEvent
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if event.Type != "tool_call" {
		t.Errorf("expected type 'tool_call', got %q", event.Type)
	}
	if event.Tool != "read" {
		t.Errorf("expected tool 'read', got %q", event.Tool)
	}
}

func TestJSONWriter_EmitToolResult(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONWriter(&buf)

	jw.EmitToolResult("read", "tc1", "file contents", false)

	var event JSONEvent
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if event.Type != "tool_result" {
		t.Errorf("expected type 'tool_result', got %q", event.Type)
	}
	if event.IsError {
		t.Error("expected is_error=false")
	}
}

func TestJSONWriter_EmitToolResult_Error(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONWriter(&buf)

	jw.EmitToolResult("bash", "tc2", "not found", true)

	var event JSONEvent
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !event.IsError {
		t.Error("expected is_error=true")
	}
}

func TestJSONWriter_EmitUsage(t *testing.T) {
	var buf bytes.Buffer
	jw := NewJSONWriter(&buf)

	jw.EmitUsage(&provider.ChatResponse{
		Usage: provider.Usage{InputTokens: 100, OutputTokens: 50},
	}, 0.0123)

	var event JSONEvent
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if event.Type != "usage" {
		t.Errorf("expected type 'usage', got %q", event.Type)
	}
	if event.Usage == nil {
		t.Fatal("expected usage data")
	}
	if event.Usage.InputTokens != 100 {
		t.Errorf("expected 100 input tokens, got %d", event.Usage.InputTokens)
	}
	if event.Usage.TotalTokens != 150 {
		t.Errorf("expected 150 total tokens, got %d", event.Usage.TotalTokens)
	}
}

func TestFormatJSONEvent(t *testing.T) {
	event := JSONEvent{Type: "test", Content: "hello"}
	s := FormatJSONEvent(event)
	if !strings.Contains(s, `"type":"test"`) {
		t.Errorf("should contain type: %s", s)
	}
}
