package compact

import (
	"context"
	"strings"
	"testing"

	"github.com/meganerd/pi-go/internal/message"
	"github.com/meganerd/pi-go/internal/provider"
)

type mockProvider struct {
	response *provider.ChatResponse
}

func (m *mockProvider) Name() string                 { return "mock" }
func (m *mockProvider) Models() []provider.Model      { return nil }
func (m *mockProvider) Chat(_ context.Context, _ *provider.ChatRequest) (*provider.ChatResponse, error) {
	return m.response, nil
}

func TestNeedsCompaction_BelowThreshold(t *testing.T) {
	c := New(nil, "", 100000, 10)
	msgs := []message.Message{
		{Role: message.RoleUser, Content: "hello"},
	}
	if c.NeedsCompaction(msgs) {
		t.Error("short conversation should not need compaction")
	}
}

func TestNeedsCompaction_AboveThreshold(t *testing.T) {
	c := New(nil, "", 10, 2) // very low threshold
	msgs := []message.Message{
		{Role: message.RoleUser, Content: strings.Repeat("x", 100)},
		{Role: message.RoleAssistant, Content: strings.Repeat("y", 100)},
	}
	if !c.NeedsCompaction(msgs) {
		t.Error("long conversation should need compaction")
	}
}

func TestCompact_NoopWhenBelowThreshold(t *testing.T) {
	c := New(nil, "", 100000, 10)
	msgs := []message.Message{
		{Role: message.RoleUser, Content: "hello"},
	}
	result, err := c.Compact(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != len(msgs) {
		t.Errorf("noop compact should return same messages: got %d, want %d", len(result), len(msgs))
	}
}

func TestCompact_SummarizesOldMessages(t *testing.T) {
	mock := &mockProvider{
		response: &provider.ChatResponse{
			Message: message.Message{
				Role:    message.RoleAssistant,
				Content: "Summary: user asked about Go, assistant explained interfaces",
			},
		},
	}

	c := New(mock, "test-model", 10, 2) // threshold 10 tokens, keep 2 recent

	msgs := []message.Message{
		{Role: message.RoleUser, Content: strings.Repeat("old message 1 ", 20)},
		{Role: message.RoleAssistant, Content: strings.Repeat("old response 1 ", 20)},
		{Role: message.RoleUser, Content: strings.Repeat("old message 2 ", 20)},
		{Role: message.RoleAssistant, Content: strings.Repeat("old response 2 ", 20)},
		{Role: message.RoleUser, Content: "recent question"},
		{Role: message.RoleAssistant, Content: "recent answer"},
	}

	result, err := c.Compact(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}

	// Should have: 1 summary + 2 recent = 3 messages
	if len(result) != 3 {
		t.Fatalf("expected 3 messages after compaction, got %d", len(result))
	}

	// First message should be the summary
	if !strings.Contains(result[0].Content, "Summary:") {
		t.Errorf("first message should contain summary, got: %s", result[0].Content)
	}

	// Last two should be the recent messages
	if result[1].Content != "recent question" {
		t.Errorf("second message should be recent question, got: %s", result[1].Content)
	}
	if result[2].Content != "recent answer" {
		t.Errorf("third message should be recent answer, got: %s", result[2].Content)
	}
}

func TestCompact_TooFewMessages(t *testing.T) {
	c := New(nil, "", 10, 10) // keep 10 but only 3 messages

	msgs := []message.Message{
		{Role: message.RoleUser, Content: strings.Repeat("x", 100)},
		{Role: message.RoleAssistant, Content: strings.Repeat("y", 100)},
		{Role: message.RoleUser, Content: strings.Repeat("z", 100)},
	}

	result, err := c.Compact(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}

	// Can't compact because keepCount > len(msgs)
	if len(result) != len(msgs) {
		t.Errorf("should return original when too few messages: got %d, want %d", len(result), len(msgs))
	}
}

func TestCompact_Idempotent(t *testing.T) {
	c := New(nil, "", 100000, 10)
	msgs := []message.Message{
		{Role: message.RoleUser, Content: "hello"},
	}

	// Compact twice — should be the same
	r1, _ := c.Compact(context.Background(), msgs)
	r2, _ := c.Compact(context.Background(), r1)

	if len(r1) != len(r2) {
		t.Errorf("compaction should be idempotent: %d vs %d", len(r1), len(r2))
	}
}
