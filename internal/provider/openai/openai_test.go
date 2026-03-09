package openai

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

func TestOpenAIChat_RequestFormat(t *testing.T) {
	var receivedBody map[string]interface{}
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "chatcmpl-test",
			"object": "chat.completion",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Hello!",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{"prompt_tokens": 10, "completion_tokens": 5},
		})
	}))
	defer server.Close()

	p := New("test-api-key", server.URL)
	_, err := p.Chat(context.Background(), &provider.ChatRequest{
		Model: "gpt-4o",
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

	// Verify Authorization header
	auth := receivedHeaders.Get("Authorization")
	if auth != "Bearer test-api-key" {
		t.Errorf("expected Bearer auth, got: %s", auth)
	}

	// Verify tools wrapped in function type
	tools, ok := receivedBody["tools"].([]interface{})
	if !ok || len(tools) != 1 {
		t.Fatalf("expected 1 tool, got: %v", receivedBody["tools"])
	}
	tool := tools[0].(map[string]interface{})
	if tool["type"] != "function" {
		t.Errorf("expected tool type 'function', got: %v", tool["type"])
	}
	fn := tool["function"].(map[string]interface{})
	if _, has := fn["parameters"]; !has {
		t.Error("expected parameters field in function definition")
	}
}

func TestOpenAIChat_ToolCallsResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "chatcmpl-test",
			"object": "chat.completion",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": nil,
						"tool_calls": []map[string]interface{}{
							{
								"id":   "call_abc123",
								"type": "function",
								"function": map[string]interface{}{
									"name":      "read",
									"arguments": `{"path":"/tmp/test.txt"}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
			"usage": map[string]interface{}{"prompt_tokens": 20, "completion_tokens": 15},
		})
	}))
	defer server.Close()

	p := New("test-key", server.URL)
	resp, err := p.Chat(context.Background(), &provider.ChatRequest{
		Model:    "gpt-4o",
		Messages: []message.Message{{Role: message.RoleUser, Content: "Read /tmp/test.txt"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have tool calls
	if !resp.Message.HasToolCalls() {
		t.Fatal("expected tool calls in response")
	}
	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Message.ToolCalls))
	}

	tc := resp.Message.ToolCalls[0]
	if tc.ID != "call_abc123" {
		t.Errorf("expected tool call ID call_abc123, got: %s", tc.ID)
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

	// Content should be empty when only tool calls
	if resp.Message.Content != "" {
		t.Errorf("expected empty content, got: %q", resp.Message.Content)
	}
}
