// Package openai implements the OpenAI Chat Completions API provider.
// Also works with OpenAI-compatible APIs (OpenRouter, local models).
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/meganerd/pi-go/internal/message"
	"github.com/meganerd/pi-go/internal/provider"
)

const defaultBaseURL = "https://api.openai.com"

// OpenAI implements the provider.Provider interface for the OpenAI Chat Completions API.
type OpenAI struct {
	apiKey  string
	baseURL string
	client  *http.Client
	name    string // "openai" or "openrouter" etc.
}

// New creates a new OpenAI-compatible provider.
func New(apiKey string, baseURL string) *OpenAI {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &OpenAI{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{},
		name:    "openai",
	}
}

// NewOpenRouter creates an OpenRouter provider (OpenAI-compatible).
func NewOpenRouter(apiKey string) *OpenAI {
	return &OpenAI{
		apiKey:  apiKey,
		baseURL: "https://openrouter.ai/api",
		client:  &http.Client{},
		name:    "openrouter",
	}
}

func (o *OpenAI) Name() string { return o.name }

func (o *OpenAI) Models() []provider.Model {
	return []provider.Model{
		{ID: "gpt-4o", Name: "GPT-4o", Provider: o.name, ContextWindow: 128000},
	}
}

// --- Wire types ---

type apiRequest struct {
	Model    string       `json:"model"`
	Messages []apiMessage `json:"messages"`
	Tools    []apiTool    `json:"tools,omitempty"`
}

type apiMessage struct {
	Role       string        `json:"role"`
	Content    interface{}   `json:"content"`            // string or null
	ToolCalls  []apiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

type apiToolCall struct {
	ID       string      `json:"id"`
	Type     string      `json:"type"`
	Function apiFunction `json:"function"`
}

type apiFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type apiTool struct {
	Type     string          `json:"type"`
	Function apiToolFunction `json:"function"`
}

type apiToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type apiResponse struct {
	ID      string     `json:"id"`
	Choices []apiChoice `json:"choices"`
	Usage   apiUsage   `json:"usage"`
}

type apiChoice struct {
	Index        int        `json:"index"`
	Message      apiMessage `json:"message"`
	FinishReason string     `json:"finish_reason"`
}

type apiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// Chat sends a request to the OpenAI Chat Completions API.
func (o *OpenAI) Chat(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	apiReq := o.buildRequest(req)

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

	httpResp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai API error %d: %s", httpResp.StatusCode, string(respBody))
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return o.parseResponse(&apiResp), nil
}

func (o *OpenAI) buildRequest(req *provider.ChatRequest) *apiRequest {
	apiReq := &apiRequest{
		Model: req.Model,
	}

	// Add system message if present
	if req.SystemPrompt != "" {
		apiReq.Messages = append(apiReq.Messages, apiMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	// Convert messages
	for _, msg := range req.Messages {
		apiReq.Messages = append(apiReq.Messages, convertMessageToAPI(msg))
	}

	// Convert tools
	for _, t := range req.Tools {
		apiReq.Tools = append(apiReq.Tools, apiTool{
			Type: "function",
			Function: apiToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Schema,
			},
		})
	}

	return apiReq
}

func convertMessageToAPI(msg message.Message) apiMessage {
	switch msg.Role {
	case message.RoleTool:
		return apiMessage{
			Role:       "tool",
			Content:    msg.ToolResult.Content,
			ToolCallID: msg.ToolResult.ToolCallID,
		}

	case message.RoleAssistant:
		apiMsg := apiMessage{
			Role:    "assistant",
			Content: msg.Content,
		}
		if msg.Content == "" {
			apiMsg.Content = nil
		}
		for _, tc := range msg.ToolCalls {
			args := string(tc.Input)
			apiMsg.ToolCalls = append(apiMsg.ToolCalls, apiToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: apiFunction{
					Name:      tc.Name,
					Arguments: args,
				},
			})
		}
		return apiMsg

	default:
		return apiMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}
}

func (o *OpenAI) parseResponse(resp *apiResponse) *provider.ChatResponse {
	if len(resp.Choices) == 0 {
		return &provider.ChatResponse{
			Message: message.Message{Role: message.RoleAssistant},
		}
	}

	choice := resp.Choices[0]
	msg := message.Message{
		ID:   resp.ID,
		Role: message.RoleAssistant,
	}

	// Extract content
	if choice.Message.Content != nil {
		if s, ok := choice.Message.Content.(string); ok {
			msg.Content = s
		}
	}

	// Extract tool calls
	for _, tc := range choice.Message.ToolCalls {
		msg.ToolCalls = append(msg.ToolCalls, message.ToolCall{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		})
	}

	return &provider.ChatResponse{
		Message: msg,
		Usage: provider.Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		},
	}
}
