package session

import (
	"fmt"
	"strings"

	"github.com/meganerd/pi-go/internal/message"
)

// ExportMarkdown converts a message history to a readable markdown format.
// Each message is rendered with a role header and content.
func ExportMarkdown(msgs []message.Message) string {
	var b strings.Builder

	b.WriteString("# Session Export\n\n")

	for _, msg := range msgs {
		switch msg.Role {
		case message.RoleUser:
			b.WriteString("## User\n\n")
			b.WriteString(msg.Content)
			b.WriteString("\n\n")

		case message.RoleAssistant:
			b.WriteString("## Assistant\n\n")
			if msg.Content != "" {
				b.WriteString(msg.Content)
				b.WriteString("\n\n")
			}
			for _, tc := range msg.ToolCalls {
				fmt.Fprintf(&b, "**Tool call:** `%s`\n```json\n%s\n```\n\n", tc.Name, string(tc.Input))
			}

		case message.RoleTool:
			if msg.ToolResult != nil {
				label := "Tool result"
				if msg.ToolResult.IsError {
					label = "Tool error"
				}
				fmt.Fprintf(&b, "**%s** (`%s`):\n```\n%s\n```\n\n",
					label, msg.ToolResult.ToolCallID, msg.ToolResult.Content)
			}

		case message.RoleSystem:
			b.WriteString("## System\n\n")
			b.WriteString(msg.Content)
			b.WriteString("\n\n")
		}
	}

	return b.String()
}

// TreeView returns a text representation of the session's branch structure.
// Each message is shown with its ID, role, and a content preview.
func TreeView(msgs []message.Message) string {
	if len(msgs) == 0 {
		return "(empty session)"
	}

	var b strings.Builder
	for i, msg := range msgs {
		prefix := "├─"
		if i == len(msgs)-1 {
			prefix = "└─"
		}
		preview := msg.Content
		if len(preview) > 60 {
			preview = preview[:57] + "..."
		}
		preview = strings.ReplaceAll(preview, "\n", " ")
		fmt.Fprintf(&b, "%s [%s] %s: %s\n", prefix, msg.ID, msg.Role, preview)
	}
	return b.String()
}
