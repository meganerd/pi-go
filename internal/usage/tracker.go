// Package usage tracks cumulative token consumption and estimated cost across a session.
package usage

import (
	"fmt"
	"sync"
)

// Pricing holds per-model cost rates in USD per 1K tokens.
type Pricing struct {
	InputPer1K  float64
	OutputPer1K float64
}

// Known pricing for common models (USD per 1K tokens).
var knownPricing = map[string]Pricing{
	// Anthropic
	"claude-sonnet-4-20250514": {InputPer1K: 0.003, OutputPer1K: 0.015},
	"claude-opus-4-20250514":   {InputPer1K: 0.015, OutputPer1K: 0.075},
	"claude-haiku-3-5-20241022": {InputPer1K: 0.0008, OutputPer1K: 0.004},
	// OpenAI
	"gpt-4o":      {InputPer1K: 0.0025, OutputPer1K: 0.01},
	"gpt-4o-mini": {InputPer1K: 0.00015, OutputPer1K: 0.0006},
	// OpenRouter passes through model pricing — use the underlying model name
}

// Tracker accumulates token usage and estimates cost.
type Tracker struct {
	mu          sync.Mutex
	model       string
	pricing     Pricing
	totalInput  int
	totalOutput int
	calls       int
	budget      int // max context tokens (0 = no budget tracking)
}

// New creates a usage tracker for the given model.
func New(model string) *Tracker {
	pricing, ok := knownPricing[model]
	if !ok {
		// Unknown model — track tokens but can't estimate cost
		pricing = Pricing{}
	}
	return &Tracker{
		model:   model,
		pricing: pricing,
	}
}

// SetBudget sets the context token budget for remaining-capacity display.
func (t *Tracker) SetBudget(maxContextTokens int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.budget = maxContextTokens
}

// Add records a single LLM call's token consumption.
func (t *Tracker) Add(inputTokens, outputTokens int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.totalInput += inputTokens
	t.totalOutput += outputTokens
	t.calls++
}

// Stats returns the current cumulative statistics.
func (t *Tracker) Stats() Stats {
	t.mu.Lock()
	defer t.mu.Unlock()
	return Stats{
		Model:        t.model,
		InputTokens:  t.totalInput,
		OutputTokens: t.totalOutput,
		TotalTokens:  t.totalInput + t.totalOutput,
		Calls:        t.calls,
		EstCost:      t.estimateCost(),
		HasPricing:   t.pricing.InputPer1K > 0 || t.pricing.OutputPer1K > 0,
		Budget:       t.budget,
	}
}

func (t *Tracker) estimateCost() float64 {
	inputCost := float64(t.totalInput) / 1000.0 * t.pricing.InputPer1K
	outputCost := float64(t.totalOutput) / 1000.0 * t.pricing.OutputPer1K
	return inputCost + outputCost
}

// Stats holds cumulative usage statistics.
type Stats struct {
	Model        string
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	Calls        int
	EstCost      float64
	HasPricing   bool
	Budget       int // max context tokens (0 = no budget)
}

// String returns a human-readable summary.
func (s Stats) String() string {
	base := fmt.Sprintf("%d tokens (%d in / %d out) across %d calls",
		s.TotalTokens, s.InputTokens, s.OutputTokens, s.Calls)
	if s.HasPricing {
		base = fmt.Sprintf("%s — est. $%.4f", base, s.EstCost)
	}
	if s.Budget > 0 {
		remaining := s.Budget - s.TotalTokens
		pct := float64(s.TotalTokens) / float64(s.Budget) * 100
		if remaining < 0 {
			remaining = 0
		}
		base = fmt.Sprintf("%s | budget: %d/%d (%.0f%% used)", base, s.TotalTokens, s.Budget, pct)
		_ = remaining // used for display logic above
	}
	return base
}
