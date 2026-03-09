package message

import (
	"encoding/json"
	"testing"
)

func TestRoleConstants(t *testing.T) {
	tests := []struct {
		role Role
		want string
	}{
		{RoleUser, "user"},
		{RoleAssistant, "assistant"},
		{RoleSystem, "system"},
		{RoleTool, "tool"},
	}
	for _, tt := range tests {
		if string(tt.role) != tt.want {
			t.Errorf("role %v = %q, want %q", tt.role, string(tt.role), tt.want)
		}
	}
}

func TestHasToolCalls_Empty(t *testing.T) {
	msg := &Message{Role: RoleAssistant, Content: "hello"}
	if msg.HasToolCalls() {
		t.Error("empty ToolCalls should return false")
	}
}

func TestHasToolCalls_WithCalls(t *testing.T) {
	msg := &Message{
		Role: RoleAssistant,
		ToolCalls: []ToolCall{
			{ID: "tc1", Name: "read", Input: json.RawMessage(`{"path":"/tmp"}`)},
		},
	}
	if !msg.HasToolCalls() {
		t.Error("non-empty ToolCalls should return true")
	}
}

func TestMessage_JSONRoundTrip(t *testing.T) {
	original := Message{
		ID:       "msg-1",
		ParentID: "msg-0",
		Role:     RoleAssistant,
		Content:  "Here is the file",
		ToolCalls: []ToolCall{
			{ID: "tc-1", Name: "read", Input: json.RawMessage(`{"path":"main.go"}`)},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, original.ID)
	}
	if decoded.ParentID != original.ParentID {
		t.Errorf("ParentID = %q, want %q", decoded.ParentID, original.ParentID)
	}
	if decoded.Role != original.Role {
		t.Errorf("Role = %q, want %q", decoded.Role, original.Role)
	}
	if decoded.Content != original.Content {
		t.Errorf("Content = %q, want %q", decoded.Content, original.Content)
	}
	if len(decoded.ToolCalls) != 1 {
		t.Fatalf("ToolCalls len = %d, want 1", len(decoded.ToolCalls))
	}
	if decoded.ToolCalls[0].Name != "read" {
		t.Errorf("ToolCall name = %q, want read", decoded.ToolCalls[0].Name)
	}
}

func TestToolResultMsg_JSONRoundTrip(t *testing.T) {
	msg := Message{
		Role: RoleTool,
		ToolResult: &ToolResultMsg{
			ToolCallID: "tc-1",
			Content:    "file contents here",
			IsError:    false,
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ToolResult == nil {
		t.Fatal("ToolResult should not be nil")
	}
	if decoded.ToolResult.ToolCallID != "tc-1" {
		t.Errorf("ToolCallID = %q, want tc-1", decoded.ToolResult.ToolCallID)
	}
	if decoded.ToolResult.IsError {
		t.Error("IsError should be false")
	}
}

func TestToolResultMsg_Error(t *testing.T) {
	msg := Message{
		Role: RoleTool,
		ToolResult: &ToolResultMsg{
			ToolCallID: "tc-2",
			Content:    "file not found",
			IsError:    true,
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !decoded.ToolResult.IsError {
		t.Error("IsError should be true")
	}
}

func TestMessage_OmitEmpty(t *testing.T) {
	msg := Message{Role: RoleUser, Content: "hello"}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}

	// These fields should be omitted (only those with omitempty tag)
	for _, field := range []string{"parent_id", "tool_calls", "tool_result"} {
		if _, ok := raw[field]; ok {
			t.Errorf("field %q should be omitted when empty", field)
		}
	}
}
