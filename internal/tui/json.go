package tui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/meganerd/pi-go/internal/message"
	"github.com/meganerd/pi-go/internal/provider"
)

// JSONEvent represents a single event in JSON output mode.
type JSONEvent struct {
	Type    string          `json:"type"`
	Role    string          `json:"role,omitempty"`
	Content string          `json:"content,omitempty"`
	Tool    string          `json:"tool,omitempty"`
	ToolID  string          `json:"tool_id,omitempty"`
	IsError bool            `json:"is_error,omitempty"`
	Usage   *JSONUsageEvent `json:"usage,omitempty"`
}

// JSONUsageEvent holds token usage data for JSON output.
type JSONUsageEvent struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalTokens  int     `json:"total_tokens"`
	EstCost      float64 `json:"est_cost,omitempty"`
}

// JSONWriter outputs events as JSON lines to the given writer.
type JSONWriter struct {
	w   io.Writer
	enc *json.Encoder
}

// NewJSONWriter creates a JSON output writer. If w is nil, os.Stdout is used.
func NewJSONWriter(w io.Writer) *JSONWriter {
	if w == nil {
		w = os.Stdout
	}
	return &JSONWriter{
		w:   w,
		enc: json.NewEncoder(w),
	}
}

// EmitMessage writes a message event.
func (j *JSONWriter) EmitMessage(msg message.Message) {
	event := JSONEvent{
		Type:    "message",
		Role:    string(msg.Role),
		Content: msg.Content,
	}
	_ = j.enc.Encode(event)
}

// EmitToolCall writes a tool call event.
func (j *JSONWriter) EmitToolCall(name, toolID string, input json.RawMessage) {
	event := JSONEvent{
		Type:    "tool_call",
		Tool:    name,
		ToolID:  toolID,
		Content: string(input),
	}
	_ = j.enc.Encode(event)
}

// EmitToolResult writes a tool result event.
func (j *JSONWriter) EmitToolResult(name, toolID, content string, isError bool) {
	event := JSONEvent{
		Type:    "tool_result",
		Tool:    name,
		ToolID:  toolID,
		Content: content,
		IsError: isError,
	}
	_ = j.enc.Encode(event)
}

// EmitUsage writes a usage summary event.
func (j *JSONWriter) EmitUsage(resp *provider.ChatResponse, estCost float64) {
	event := JSONEvent{
		Type: "usage",
		Usage: &JSONUsageEvent{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			TotalTokens:  resp.Usage.InputTokens + resp.Usage.OutputTokens,
			EstCost:      estCost,
		},
	}
	_ = j.enc.Encode(event)
}

// FormatJSONEvent formats a JSONEvent as a JSON string (for testing).
func FormatJSONEvent(event JSONEvent) string {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Sprintf(`{"type":"error","content":"%v"}`, err)
	}
	return string(data)
}
