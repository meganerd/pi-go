// Package provider defines the LLM provider interface for pi-go.
package provider

import (
	"context"
	"encoding/json"

	"github.com/meganerd/pi-go/internal/message"
)

// Provider abstracts communication with an LLM API.
type Provider interface {
	// Name returns the provider's identifier (e.g., "anthropic", "openai").
	Name() string

	// Chat sends a conversation and returns the complete response.
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// Models returns the list of available models for this provider.
	Models() []Model
}

// StreamProvider is an optional interface for providers that support streaming.
// Use CanStream to check if a provider supports streaming.
type StreamProvider interface {
	Provider
	// StreamChat sends a conversation and returns a channel of streamed events.
	StreamChat(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)
}

// CanStream returns true if the provider supports streaming.
func CanStream(p Provider) bool {
	_, ok := p.(StreamProvider)
	return ok
}

// ChatRequest holds the input for an LLM call.
type ChatRequest struct {
	Model        string            `json:"model"`
	Messages     []message.Message `json:"messages"`
	SystemPrompt string            `json:"system_prompt,omitempty"`
	Tools        []ToolDef         `json:"tools,omitempty"`
	MaxTokens    int               `json:"max_tokens,omitempty"`
	Temperature  float64           `json:"temperature,omitempty"`
}

// ToolDef describes a tool available to the LLM.
type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"` // JSON Schema
}

// ChatResponse holds the complete response from an LLM call.
type ChatResponse struct {
	Message message.Message `json:"message"`
	Usage   Usage           `json:"usage"`
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Model describes an available LLM model.
type Model struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Provider       string  `json:"provider"`
	ContextWindow  int     `json:"context_window"`
	CostPerKInput  float64 `json:"cost_per_k_input"`
	CostPerKOutput float64 `json:"cost_per_k_output"`
}

// Registry maps provider names to constructor functions.
type Registry struct {
	providers map[string]func(apiKey string, baseURL string) Provider
}

// NewRegistry creates an empty provider registry.
func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]func(apiKey string, baseURL string) Provider)}
}

// Register adds a provider constructor to the registry.
func (r *Registry) Register(name string, constructor func(apiKey string, baseURL string) Provider) {
	r.providers[name] = constructor
}

// Get returns a provider instance by name, or nil if not registered.
func (r *Registry) Get(name string, apiKey string, baseURL string) Provider {
	constructor, ok := r.providers[name]
	if !ok {
		return nil
	}
	return constructor(apiKey, baseURL)
}

// Names returns the list of registered provider names.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}
