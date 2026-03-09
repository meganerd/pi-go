// Package message defines conversation message types for pi-go.
package message

// Role identifies the sender of a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// Message represents a single conversation message.
type Message struct {
	ID       string      `json:"id"`
	ParentID string      `json:"parent_id,omitempty"`
	Role     Role        `json:"role"`
	Content  string      `json:"content"`
	ToolUse  *ToolUse    `json:"tool_use,omitempty"`
	ToolResult *ToolResultMsg `json:"tool_result,omitempty"`
}

// ToolUse represents an LLM's request to invoke a tool.
type ToolUse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input string `json:"input"` // Raw JSON
}

// ToolResultMsg represents the result of a tool invocation.
type ToolResultMsg struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
}
