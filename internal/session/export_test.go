package session

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/meganerd/pi-go/internal/message"
)

func TestExportMarkdown_Basic(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: "Hello"},
		{Role: message.RoleAssistant, Content: "Hi there!"},
	}

	md := ExportMarkdown(msgs)

	if !strings.Contains(md, "# Session Export") {
		t.Error("should contain header")
	}
	if !strings.Contains(md, "## User") {
		t.Error("should contain User header")
	}
	if !strings.Contains(md, "Hello") {
		t.Error("should contain user message")
	}
	if !strings.Contains(md, "## Assistant") {
		t.Error("should contain Assistant header")
	}
	if !strings.Contains(md, "Hi there!") {
		t.Error("should contain assistant message")
	}
}

func TestExportMarkdown_ToolCalls(t *testing.T) {
	msgs := []message.Message{
		{
			Role: message.RoleAssistant,
			ToolCalls: []message.ToolCall{
				{ID: "tc1", Name: "read", Input: json.RawMessage(`{"path":"/tmp/test.txt"}`)},
			},
		},
		{
			Role: message.RoleTool,
			ToolResult: &message.ToolResultMsg{
				ToolCallID: "tc1",
				Content:    "file contents",
			},
		},
	}

	md := ExportMarkdown(msgs)

	if !strings.Contains(md, "Tool call") {
		t.Error("should contain tool call")
	}
	if !strings.Contains(md, "`read`") {
		t.Error("should contain tool name")
	}
	if !strings.Contains(md, "Tool result") {
		t.Error("should contain tool result")
	}
}

func TestExportMarkdown_ToolError(t *testing.T) {
	msgs := []message.Message{
		{
			Role: message.RoleTool,
			ToolResult: &message.ToolResultMsg{
				ToolCallID: "tc1",
				Content:    "file not found",
				IsError:    true,
			},
		},
	}

	md := ExportMarkdown(msgs)
	if !strings.Contains(md, "Tool error") {
		t.Error("should contain Tool error for error results")
	}
}

func TestExportMarkdown_Empty(t *testing.T) {
	md := ExportMarkdown(nil)
	if !strings.Contains(md, "# Session Export") {
		t.Error("should still have header for empty session")
	}
}

func TestTreeView_Basic(t *testing.T) {
	msgs := []message.Message{
		{ID: "1", Role: message.RoleUser, Content: "Hello"},
		{ID: "2", Role: message.RoleAssistant, Content: "Hi"},
	}

	tree := TreeView(msgs)

	if !strings.Contains(tree, "[1]") {
		t.Error("should contain message ID")
	}
	if !strings.Contains(tree, "user") {
		t.Error("should contain role")
	}
	if !strings.Contains(tree, "└─") {
		t.Error("should contain tree chars")
	}
}

func TestTreeView_LongContent(t *testing.T) {
	long := strings.Repeat("a", 100)
	msgs := []message.Message{
		{ID: "1", Role: message.RoleUser, Content: long},
	}

	tree := TreeView(msgs)

	if strings.Contains(tree, strings.Repeat("a", 100)) {
		t.Error("should truncate long content")
	}
	if !strings.Contains(tree, "...") {
		t.Error("should contain ellipsis for truncated content")
	}
}

func TestTreeView_Empty(t *testing.T) {
	tree := TreeView(nil)
	if tree != "(empty session)" {
		t.Errorf("empty tree should show message, got: %q", tree)
	}
}
