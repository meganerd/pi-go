// Package compact provides context compaction for long conversations.
// When messages exceed a token threshold, older messages are summarized
// into a single system-role message, preserving recent context.
package compact

import (
	"context"
	"fmt"

	"github.com/meganerd/pi-go/internal/message"
	"github.com/meganerd/pi-go/internal/provider"
	"github.com/meganerd/pi-go/internal/token"
)

const (
	// DefaultMaxTokens is the default context token limit before compaction triggers.
	DefaultMaxTokens = 100000
	// DefaultKeepRecent is the number of recent messages to preserve uncompacted.
	DefaultKeepRecent = 10
)

const compactionPrompt = `Summarize the following conversation history concisely. Preserve:
- Key decisions made
- Important file paths and code changes
- Tool calls and their outcomes
- Any errors or issues encountered
- The user's original request and current progress

Be concise but complete. Use bullet points. This summary will replace the conversation history.`

// Compactor manages context compaction for a conversation.
type Compactor struct {
	provider  provider.Provider
	model     string
	maxTokens int
	keepCount int
}

// New creates a Compactor. If maxTokens is 0, DefaultMaxTokens is used.
// If keepCount is 0, DefaultKeepRecent is used.
func New(p provider.Provider, model string, maxTokens, keepCount int) *Compactor {
	if maxTokens == 0 {
		maxTokens = DefaultMaxTokens
	}
	if keepCount == 0 {
		keepCount = DefaultKeepRecent
	}
	return &Compactor{
		provider:  p,
		model:     model,
		maxTokens: maxTokens,
		keepCount: keepCount,
	}
}

// NeedsCompaction returns true if the messages exceed the token threshold.
func (c *Compactor) NeedsCompaction(msgs []message.Message) bool {
	return token.EstimateMessages(msgs) > c.maxTokens
}

// Compact summarizes older messages, keeping recent ones intact.
// Returns the compacted message slice. If compaction isn't needed, returns msgs unchanged.
func (c *Compactor) Compact(ctx context.Context, msgs []message.Message) ([]message.Message, error) {
	if !c.NeedsCompaction(msgs) {
		return msgs, nil
	}

	// Split into old (to summarize) and recent (to keep)
	splitIdx := len(msgs) - c.keepCount
	if splitIdx <= 0 {
		// Not enough messages to compact — keep all
		return msgs, nil
	}

	old := msgs[:splitIdx]
	recent := msgs[splitIdx:]

	// Build summary request
	summaryContent := "Conversation history to summarize:\n\n"
	for _, msg := range old {
		summaryContent += fmt.Sprintf("[%s]: %s\n", msg.Role, msg.Content)
		for _, tc := range msg.ToolCalls {
			summaryContent += fmt.Sprintf("  [tool_call: %s(%s)]\n", tc.Name, string(tc.Input))
		}
		if msg.ToolResult != nil {
			prefix := "result"
			if msg.ToolResult.IsError {
				prefix = "error"
			}
			summaryContent += fmt.Sprintf("  [tool_%s: %s]\n", prefix, truncate(msg.ToolResult.Content, 200))
		}
	}

	req := &provider.ChatRequest{
		Model:        c.model,
		SystemPrompt: compactionPrompt,
		Messages: []message.Message{
			{Role: message.RoleUser, Content: summaryContent},
		},
		MaxTokens: 2048,
	}

	resp, err := c.provider.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("compaction summary: %w", err)
	}

	// Build compacted conversation: summary + recent messages
	compacted := make([]message.Message, 0, 1+len(recent))
	compacted = append(compacted, message.Message{
		Role:    message.RoleUser,
		Content: "[Previous conversation summary]\n\n" + resp.Message.Content,
	})
	compacted = append(compacted, recent...)

	return compacted, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
