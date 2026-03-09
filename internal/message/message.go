// Package message defines conversation message types for pi-go.
package message

import "encoding/json"

// Role identifies the sender of a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// Message represents a single conversation message.
// This is the internal representation — providers convert their wire formats to/from this.
type Message struct {
	ID       string `json:"id"`
	ParentID string `json:"parent_id,omitempty"`
	Role     Role   `json:"role"`

	// Content holds the text content of the message. May be empty when only tool calls are present.
	Content string `json:"content,omitempty"`

	// ToolCalls holds tool invocations requested by the assistant. Nil for non-tool-calling messages.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// ToolResult holds the result of a tool invocation (for role=tool messages).
	ToolResult *ToolResultMsg `json:"tool_result,omitempty"`
}

// ToolCall represents an LLM's request to invoke a tool.
type ToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"` // Raw JSON input for the tool
}

// ToolResultMsg represents the result of a tool invocation.
type ToolResultMsg struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error,omitempty"`
}

// HasToolCalls returns true if this message contains tool call requests.
func (m *Message) HasToolCalls() bool {
	return len(m.ToolCalls) > 0
}
