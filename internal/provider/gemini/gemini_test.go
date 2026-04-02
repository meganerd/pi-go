package gemini

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

func TestGeminiChat_RequestFormat(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"role":  "model",
						"parts": []map[string]interface{}{{"text": "Hello!"}},
					},
					"finishReason": "STOP",
				},
			},
			"usageMetadata": map[string]interface{}{
				"promptTokenCount":     10,
				"candidatesTokenCount": 5,
			},
		})
	}))
	defer server.Close()

	g := New("test-key", server.URL)
	resp, err := g.Chat(context.Background(), &provider.ChatRequest{
		Model:        "gemini-2.5-flash",
		SystemPrompt: "You are helpful.",
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

	// Verify system instruction was set.
	si, ok := receivedBody["systemInstruction"].(map[string]interface{})
	if !ok {
		t.Fatal("expected systemInstruction in request")
	}
	parts := si["parts"].([]interface{})
	part0 := parts[0].(map[string]interface{})
	if part0["text"] != "You are helpful." {
		t.Errorf("expected system prompt, got: %v", part0["text"])
	}

	// Verify contents has user message.
	contents := receivedBody["contents"].([]interface{})
	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}
	content0 := contents[0].(map[string]interface{})
	if content0["role"] != "user" {
		t.Errorf("expected user role, got: %v", content0["role"])
	}

	// Verify tools present.
	tools := receivedBody["tools"].([]interface{})
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool group, got %d", len(tools))
	}

	// Verify response parsed correctly.
	if resp.Message.Content != "Hello!" {
		t.Errorf("expected content 'Hello!', got: %q", resp.Message.Content)
	}
	if resp.Usage.InputTokens != 10 {
		t.Errorf("expected 10 input tokens, got %d", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 5 {
		t.Errorf("expected 5 output tokens, got %d", resp.Usage.OutputTokens)
	}
}

func TestGeminiChat_FunctionCallResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"role": "model",
						"parts": []map[string]interface{}{
							{
								"functionCall": map[string]interface{}{
									"name": "read",
									"args": map[string]interface{}{"path": "/tmp/test.txt"},
								},
							},
						},
					},
					"finishReason": "STOP",
				},
			},
		})
	}))
	defer server.Close()

	g := New("test-key", server.URL)
	resp, err := g.Chat(context.Background(), &provider.ChatRequest{
		Model:    "gemini-2.5-flash",
		Messages: []message.Message{{Role: message.RoleUser, Content: "Read /tmp/test.txt"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Message.HasToolCalls() {
		t.Fatal("expected tool calls in response")
	}
	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Message.ToolCalls))
	}

	tc := resp.Message.ToolCalls[0]
	if tc.Name != "read" {
		t.Errorf("expected tool name 'read', got: %s", tc.Name)
	}
	if tc.ID != "gemini_read" {
		t.Errorf("expected ID 'gemini_read', got: %s", tc.ID)
	}

	var input map[string]interface{}
	if err := json.Unmarshal(tc.Input, &input); err != nil {
		t.Fatalf("tool call input not valid JSON: %v", err)
	}
	if input["path"] != "/tmp/test.txt" {
		t.Errorf("expected path /tmp/test.txt, got: %v", input["path"])
	}
}

func TestGemini_ParseStreamChunk_Text(t *testing.T) {
	g := &Gemini{}

	data := `{"candidates":[{"content":{"role":"model","parts":[{"text":"Hello world"}]},"finishReason":"STOP"}]}`
	events := g.parseStreamChunk(data)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "text" || events[0].Content != "Hello world" {
		t.Errorf("expected text event with 'Hello world', got: %+v", events[0])
	}
}

func TestGemini_ParseStreamChunk_FunctionCall(t *testing.T) {
	g := &Gemini{}

	data := `{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"bash","args":{"command":"ls"}}}]},"finishReason":"STOP"}]}`
	events := g.parseStreamChunk(data)

	// Should emit: tool_use_start, tool_use_delta, tool_use_end
	if len(events) != 3 {
		t.Fatalf("expected 3 events (start/delta/end), got %d: %+v", len(events), events)
	}
	if events[0].Type != "tool_use_start" || events[0].ToolName != "bash" {
		t.Errorf("expected tool_use_start for bash, got: %+v", events[0])
	}
	if events[1].Type != "tool_use_delta" {
		t.Errorf("expected tool_use_delta, got: %+v", events[1])
	}
	if events[2].Type != "tool_use_end" {
		t.Errorf("expected tool_use_end, got: %+v", events[2])
	}
}

func TestGemini_ParseStreamChunk_InvalidJSON(t *testing.T) {
	g := &Gemini{}
	events := g.parseStreamChunk("not json")
	if events != nil {
		t.Errorf("expected nil events for invalid JSON, got: %+v", events)
	}
}

func TestGemini_ConvertMessages_ToolResult(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: "Read a file"},
		{
			Role: message.RoleAssistant,
			ToolCalls: []message.ToolCall{
				{ID: "gemini_read", Name: "read", Input: json.RawMessage(`{"path":"/tmp/test.txt"}`)},
			},
		},
		{
			Role: message.RoleTool,
			ToolResult: &message.ToolResultMsg{
				ToolCallID: "gemini_read",
				Content:    "file contents here",
			},
		},
	}

	contents := convertMessages(msgs)

	if len(contents) != 3 {
		t.Fatalf("expected 3 contents, got %d", len(contents))
	}

	// User message
	if contents[0].Role != "user" {
		t.Errorf("expected user role, got: %s", contents[0].Role)
	}

	// Model with function call
	if contents[1].Role != "model" {
		t.Errorf("expected model role, got: %s", contents[1].Role)
	}
	if contents[1].Parts[0].FunctionCall == nil {
		t.Error("expected function call in model parts")
	}

	// Function response (as user role)
	if contents[2].Role != "user" {
		t.Errorf("expected user role for function response, got: %s", contents[2].Role)
	}
	if contents[2].Parts[0].FunctionResponse == nil {
		t.Error("expected function response in parts")
	}
	if contents[2].Parts[0].FunctionResponse.Name != "read" {
		t.Errorf("expected function name 'read', got: %s", contents[2].Parts[0].FunctionResponse.Name)
	}
}

func TestGemini_EmptyCandidates(t *testing.T) {
	g := &Gemini{}
	resp := g.parseResponse(&apiResponse{})

	if resp.Message.Role != message.RoleAssistant {
		t.Errorf("expected assistant role, got: %s", resp.Message.Role)
	}
	if resp.Message.Content != "" {
		t.Errorf("expected empty content, got: %q", resp.Message.Content)
	}
}

func TestGeminiName(t *testing.T) {
	g := New("key", "")
	if g.Name() != "gemini" {
		t.Errorf("expected name 'gemini', got: %q", g.Name())
	}
}

func TestGeminiModels(t *testing.T) {
	g := New("key", "")
	models := g.Models()
	if len(models) < 2 {
		t.Errorf("expected at least 2 models, got %d", len(models))
	}
}
