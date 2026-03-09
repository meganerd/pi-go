package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/meganerd/pi-go/internal/message"
	"github.com/meganerd/pi-go/internal/provider"
)

func TestAnthropicChat_RequestFormat(t *testing.T) {
	var receivedBody map[string]interface{}
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)

		// Return a simple text response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "msg_test",
			"type": "message",
			"role": "assistant",
			"content": []map[string]interface{}{
				{"type": "text", "text": "Hello!"},
			},
			"stop_reason": "end_turn",
			"usage":       map[string]interface{}{"input_tokens": 10, "output_tokens": 5},
		})
	}))
	defer server.Close()

	p := New("test-api-key", server.URL)
	_, err := p.Chat(context.Background(), &provider.ChatRequest{
		Model:        "claude-sonnet-4-20250514",
		SystemPrompt: "You are helpful.",
		MaxTokens:    1024,
		Messages: []message.Message{
			{Role: message.RoleUser, Content: "Hi"},
		},
		Tools: []provider.ToolDef{
			{
				Name:        "read",
				Description: "Read a file",
				Schema:      json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify headers
	if receivedHeaders.Get("X-Api-Key") != "test-api-key" {
		t.Errorf("expected x-api-key header, got: %s", receivedHeaders.Get("X-Api-Key"))
	}
	if receivedHeaders.Get("Anthropic-Version") == "" {
		t.Error("expected anthropic-version header")
	}

	// Verify request body structure
	if receivedBody["model"] != "claude-sonnet-4-20250514" {
		t.Errorf("expected model claude-sonnet-4-20250514, got: %v", receivedBody["model"])
	}
	if receivedBody["system"] != "You are helpful." {
		t.Errorf("expected system prompt, got: %v", receivedBody["system"])
	}

	// Verify tools use input_schema (not parameters)
	tools, ok := receivedBody["tools"].([]interface{})
	if !ok || len(tools) != 1 {
		t.Fatalf("expected 1 tool, got: %v", receivedBody["tools"])
	}
	tool := tools[0].(map[string]interface{})
	if _, has := tool["input_schema"]; !has {
		t.Error("expected input_schema field in tool definition")
	}
}

func TestAnthropicChat_ToolUseResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "msg_test",
			"type": "message",
			"role": "assistant",
			"content": []map[string]interface{}{
				{"type": "text", "text": "Let me read that file."},
				{
					"type":  "tool_use",
					"id":    "toolu_123",
					"name":  "read",
					"input": map[string]interface{}{"path": "/tmp/test.txt"},
				},
			},
			"stop_reason": "tool_use",
			"usage":       map[string]interface{}{"input_tokens": 20, "output_tokens": 15},
		})
	}))
	defer server.Close()

	p := New("test-key", server.URL)
	resp, err := p.Chat(context.Background(), &provider.ChatRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
		Messages:  []message.Message{{Role: message.RoleUser, Content: "Read /tmp/test.txt"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have text content
	if resp.Message.Content != "Let me read that file." {
		t.Errorf("expected text content, got: %q", resp.Message.Content)
	}

	// Should have tool calls
	if !resp.Message.HasToolCalls() {
		t.Fatal("expected tool calls in response")
	}
	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Message.ToolCalls))
	}

	tc := resp.Message.ToolCalls[0]
	if tc.ID != "toolu_123" {
		t.Errorf("expected tool call ID toolu_123, got: %s", tc.ID)
	}
	if tc.Name != "read" {
		t.Errorf("expected tool name read, got: %s", tc.Name)
	}

	// Verify input is valid JSON
	var input map[string]interface{}
	if err := json.Unmarshal(tc.Input, &input); err != nil {
		t.Fatalf("tool call input is not valid JSON: %v", err)
	}
	if input["path"] != "/tmp/test.txt" {
		t.Errorf("expected path /tmp/test.txt, got: %v", input["path"])
	}

	// Verify usage
	if resp.Usage.InputTokens != 20 || resp.Usage.OutputTokens != 15 {
		t.Errorf("unexpected usage: %+v", resp.Usage)
	}
}
