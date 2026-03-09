package tui

import (
	"fmt"
	"strings"

	"github.com/meganerd/pi-go/internal/tool"
)

// DefaultSystemPrompt builds the default system prompt with tool descriptions.
func DefaultSystemPrompt(tools []tool.Tool) string {
	var b strings.Builder
	b.WriteString("You are pi-go, an LLM-powered coding assistant. You help users with software engineering tasks including writing code, debugging, refactoring, and explaining code.\n\n")
	b.WriteString("You have access to the following tools for interacting with the local filesystem and running commands:\n\n")

	for _, t := range tools {
		fmt.Fprintf(&b, "- **%s**: %s\n", t.Name(), t.Description())
	}

	b.WriteString("\nUse tools when needed to read, write, or modify files. Be concise and direct in your responses.")
	return b.String()
}
