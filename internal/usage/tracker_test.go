package usage

import (
	"strings"
	"testing"
)

func TestTracker_NewKnownModel(t *testing.T) {
	tr := New("claude-sonnet-4-20250514")
	stats := tr.Stats()
	if !stats.HasPricing {
		t.Error("known model should have pricing")
	}
	if stats.Model != "claude-sonnet-4-20250514" {
		t.Errorf("model = %q", stats.Model)
	}
}

func TestTracker_NewUnknownModel(t *testing.T) {
	tr := New("my-custom-model")
	stats := tr.Stats()
	if stats.HasPricing {
		t.Error("unknown model should not have pricing")
	}
}

func TestTracker_Add(t *testing.T) {
	tr := New("gpt-4o")
	tr.Add(100, 50)
	tr.Add(200, 100)

	stats := tr.Stats()
	if stats.InputTokens != 300 {
		t.Errorf("input = %d, want 300", stats.InputTokens)
	}
	if stats.OutputTokens != 150 {
		t.Errorf("output = %d, want 150", stats.OutputTokens)
	}
	if stats.TotalTokens != 450 {
		t.Errorf("total = %d, want 450", stats.TotalTokens)
	}
	if stats.Calls != 2 {
		t.Errorf("calls = %d, want 2", stats.Calls)
	}
}

func TestTracker_EstCost(t *testing.T) {
	tr := New("claude-sonnet-4-20250514") // $0.003/1K in, $0.015/1K out
	tr.Add(1000, 1000)

	stats := tr.Stats()
	// Expected: (1000/1000)*0.003 + (1000/1000)*0.015 = 0.018
	if stats.EstCost < 0.017 || stats.EstCost > 0.019 {
		t.Errorf("est cost = %.6f, want ~0.018", stats.EstCost)
	}
}

func TestTracker_ZeroTokens(t *testing.T) {
	tr := New("gpt-4o")
	stats := tr.Stats()
	if stats.TotalTokens != 0 {
		t.Errorf("total = %d, want 0", stats.TotalTokens)
	}
	if stats.Calls != 0 {
		t.Errorf("calls = %d, want 0", stats.Calls)
	}
	if stats.EstCost != 0 {
		t.Errorf("cost = %f, want 0", stats.EstCost)
	}
}

func TestStats_String_WithPricing(t *testing.T) {
	tr := New("gpt-4o")
	tr.Add(500, 250)
	s := tr.Stats().String()
	if !strings.Contains(s, "750 tokens") {
		t.Errorf("should contain total tokens: %s", s)
	}
	if !strings.Contains(s, "est. $") {
		t.Errorf("should contain estimated cost: %s", s)
	}
}

func TestStats_String_WithoutPricing(t *testing.T) {
	tr := New("unknown-model")
	tr.Add(500, 250)
	s := tr.Stats().String()
	if !strings.Contains(s, "750 tokens") {
		t.Errorf("should contain total tokens: %s", s)
	}
	if strings.Contains(s, "est. $") {
		t.Errorf("should NOT contain cost estimate: %s", s)
	}
}
