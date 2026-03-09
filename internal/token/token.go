// Package token provides token estimation without external tokenizer dependencies.
package token

import (
	"github.com/meganerd/pi-go/internal/message"
)

// BytesPerToken is the approximate ratio of bytes to tokens for English text.
// This is a conservative estimate — actual ratio varies by model and language.
const BytesPerToken = 4

// Estimate returns an approximate token count for a string.
func Estimate(s string) int {
	if len(s) == 0 {
		return 0
	}
	return (len(s) + BytesPerToken - 1) / BytesPerToken // ceiling division
}

// EstimateMessage returns an approximate token count for a single message,
// including role overhead.
func EstimateMessage(msg message.Message) int {
	tokens := 4 // role + structural overhead
	tokens += Estimate(msg.Content)
	for _, tc := range msg.ToolCalls {
		tokens += Estimate(tc.Name)
		tokens += Estimate(string(tc.Input))
	}
	if msg.ToolResult != nil {
		tokens += Estimate(msg.ToolResult.Content)
	}
	return tokens
}

// EstimateMessages returns the total estimated token count for a slice of messages.
func EstimateMessages(msgs []message.Message) int {
	total := 0
	for _, msg := range msgs {
		total += EstimateMessage(msg)
	}
	return total
}
