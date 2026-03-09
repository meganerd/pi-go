// Package provider defines the LLM provider interface for pi-go.
package provider

import (
	"context"

	"github.com/meganerd/pi-go/internal/message"
)

// Provider abstracts communication with an LLM API.
type Provider interface {
	// Chat sends a conversation and returns the complete response.
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// StreamChat sends a conversation and returns a channel of streamed events.
	StreamChat(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)

	// Models returns the list of available models for this provider.
	Models() []Model
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
	Name        string `json:"name"`
	Description string `json:"description"`
	Schema      string `json:"schema"` // JSON Schema
}

// ChatResponse holds the complete response from an LLM call.
type ChatResponse struct {
	Message message.Message `json:"message"`
	Usage   Usage           `json:"usage"`
}

// StreamEvent represents a single event in a streaming response.
type StreamEvent struct {
	Type    string `json:"type"` // "text", "tool_use", "done", "error"
	Content string `json:"content,omitempty"`
	Error   error  `json:"-"`
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Model describes an available LLM model.
type Model struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Provider      string  `json:"provider"`
	ContextWindow int     `json:"context_window"`
	CostPerKInput float64 `json:"cost_per_k_input"`
	CostPerKOutput float64 `json:"cost_per_k_output"`
}
