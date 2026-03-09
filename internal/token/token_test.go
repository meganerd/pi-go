package token

import (
	"encoding/json"
	"testing"

	"github.com/meganerd/pi-go/internal/message"
)

func TestEstimate_Empty(t *testing.T) {
	if got := Estimate(""); got != 0 {
		t.Errorf("Estimate(\"\") = %d, want 0", got)
	}
}

func TestEstimate_Short(t *testing.T) {
	got := Estimate("hello")
	if got < 1 {
		t.Errorf("Estimate(\"hello\") = %d, want >= 1", got)
	}
}

func TestEstimate_Proportional(t *testing.T) {
	short := Estimate("hello")
	long := Estimate("hello world this is a longer string for testing token estimation")
	if long <= short {
		t.Errorf("longer string should have more tokens: short=%d, long=%d", short, long)
	}
}

func TestEstimateMessage_Empty(t *testing.T) {
	msg := message.Message{}
	got := EstimateMessage(msg)
	if got < 4 { // at least role overhead
		t.Errorf("empty message should have overhead tokens, got %d", got)
	}
}

func TestEstimateMessage_WithContent(t *testing.T) {
	msg := message.Message{
		Role:    message.RoleUser,
		Content: "Tell me about Go programming",
	}
	got := EstimateMessage(msg)
	if got < 8 {
		t.Errorf("message with content should have > 8 tokens, got %d", got)
	}
}

func TestEstimateMessage_WithToolCall(t *testing.T) {
	msg := message.Message{
		Role: message.RoleAssistant,
		ToolCalls: []message.ToolCall{
			{Name: "read", Input: json.RawMessage(`{"path": "/tmp/test.go"}`)},
		},
	}
	got := EstimateMessage(msg)
	if got < 10 {
		t.Errorf("tool call message should have > 10 tokens, got %d", got)
	}
}

func TestEstimateMessages_Sum(t *testing.T) {
	msgs := []message.Message{
		{Role: message.RoleUser, Content: "hello"},
		{Role: message.RoleAssistant, Content: "world"},
	}
	total := EstimateMessages(msgs)
	individual := EstimateMessage(msgs[0]) + EstimateMessage(msgs[1])
	if total != individual {
		t.Errorf("EstimateMessages = %d, sum of individuals = %d", total, individual)
	}
}

func TestEstimateMessages_Empty(t *testing.T) {
	if got := EstimateMessages(nil); got != 0 {
		t.Errorf("EstimateMessages(nil) = %d, want 0", got)
	}
}
