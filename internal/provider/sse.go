package provider

import (
	"bufio"
	"io"
	"strings"
)

// SSEEvent represents a single Server-Sent Event.
type SSEEvent struct {
	Event string
	Data  string
}

// ParseSSE reads SSE events from a reader and sends them on the returned channel.
// The channel is closed when the reader is exhausted or an error occurs.
func ParseSSE(r io.Reader) <-chan SSEEvent {
	ch := make(chan SSEEvent, 16)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(r)
		var event SSEEvent
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				// Empty line = event boundary
				if event.Data != "" {
					ch <- event
				}
				event = SSEEvent{}
				continue
			}
			if strings.HasPrefix(line, "event: ") {
				event.Event = strings.TrimPrefix(line, "event: ")
			} else if strings.HasPrefix(line, "data: ") {
				if event.Data != "" {
					event.Data += "\n"
				}
				event.Data += strings.TrimPrefix(line, "data: ")
			}
		}
		// Flush any remaining event
		if event.Data != "" {
			ch <- event
		}
	}()
	return ch
}

// StreamEvent represents a single event in a streaming response.
type StreamEvent struct {
	Type    string `json:"type"` // "text", "tool_use_start", "tool_use_delta", "tool_use_end", "done", "error"
	Content string `json:"content,omitempty"`

	// Tool use fields (populated for tool_use_start)
	ToolID   string `json:"tool_id,omitempty"`
	ToolName string `json:"tool_name,omitempty"`

	Error error `json:"-"`
}
