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
		Model:       t.model,
		InputTokens: t.totalInput,
		OutputTokens: t.totalOutput,
		TotalTokens: t.totalInput + t.totalOutput,
		Calls:       t.calls,
		EstCost:     t.estimateCost(),
		HasPricing:  t.pricing.InputPer1K > 0 || t.pricing.OutputPer1K > 0,
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
}

// String returns a human-readable summary.
func (s Stats) String() string {
	base := fmt.Sprintf("%d tokens (%d in / %d out) across %d calls",
		s.TotalTokens, s.InputTokens, s.OutputTokens, s.Calls)
	if s.HasPricing {
		return fmt.Sprintf("%s — est. $%.4f", base, s.EstCost)
	}
	return base
}
