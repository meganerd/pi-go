// Package anthropic implements the Anthropic Messages API provider.
package anthropic

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

const defaultBaseURL = "https://api.anthropic.com"

// Anthropic implements the provider.Provider interface for the Anthropic Messages API.
type Anthropic struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// New creates a new Anthropic provider. If baseURL is empty, the default is used.
func New(apiKey string, baseURL string) *Anthropic {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Anthropic{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

func (a *Anthropic) Name() string { return "anthropic" }

func (a *Anthropic) Models() []provider.Model {
	return []provider.Model{
		{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", Provider: "anthropic", ContextWindow: 200000},
		{ID: "claude-opus-4-20250514", Name: "Claude Opus 4", Provider: "anthropic", ContextWindow: 200000},
	}
}

// --- Wire types (API-specific JSON shapes) ---

type apiRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	System    string        `json:"system,omitempty"`
	Messages  []apiMessage  `json:"messages"`
	Tools     []apiTool     `json:"tools,omitempty"`
}

type apiMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []apiContentBlock
}

type apiContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

type apiTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type apiResponse struct {
	ID         string             `json:"id"`
	Content    []apiContentBlock  `json:"content"`
	StopReason string             `json:"stop_reason"`
	Usage      apiUsage           `json:"usage"`
}

type apiUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Chat sends a request to the Anthropic Messages API.
func (a *Anthropic) Chat(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	apiReq := a.buildRequest(req)

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Api-Key", a.apiKey)
	httpReq.Header.Set("Anthropic-Version", "2023-06-01")

	httpResp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic API error %d: %s", httpResp.StatusCode, string(respBody))
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return a.parseResponse(&apiResp), nil
}

func (a *Anthropic) buildRequest(req *provider.ChatRequest) *apiRequest {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	apiReq := &apiRequest{
		Model:     req.Model,
		MaxTokens: maxTokens,
		System:    req.SystemPrompt,
	}

	// Convert messages
	for _, msg := range req.Messages {
		apiReq.Messages = append(apiReq.Messages, convertMessageToAPI(msg))
	}

	// Convert tools
	for _, t := range req.Tools {
		apiReq.Tools = append(apiReq.Tools, apiTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Schema,
		})
	}

	return apiReq
}

func convertMessageToAPI(msg message.Message) apiMessage {
	switch msg.Role {
	case message.RoleTool:
		// Tool results become user messages with tool_result content blocks
		blocks := []apiContentBlock{
			{
				Type:      "tool_result",
				ToolUseID: msg.ToolResult.ToolCallID,
				Content:   msg.ToolResult.Content,
				IsError:   msg.ToolResult.IsError,
			},
		}
		return apiMessage{Role: "user", Content: blocks}

	case message.RoleAssistant:
		if msg.HasToolCalls() {
			var blocks []apiContentBlock
			if msg.Content != "" {
				blocks = append(blocks, apiContentBlock{Type: "text", Text: msg.Content})
			}
			for _, tc := range msg.ToolCalls {
				blocks = append(blocks, apiContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: tc.Input,
				})
			}
			return apiMessage{Role: "assistant", Content: blocks}
		}
		return apiMessage{Role: "assistant", Content: msg.Content}

	default:
		return apiMessage{Role: string(msg.Role), Content: msg.Content}
	}
}

func (a *Anthropic) parseResponse(resp *apiResponse) *provider.ChatResponse {
	msg := message.Message{
		ID:   resp.ID,
		Role: message.RoleAssistant,
	}

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			msg.Content += block.Text
		case "tool_use":
			input := block.Input
			if input == nil {
				input = json.RawMessage(`{}`)
			}
			msg.ToolCalls = append(msg.ToolCalls, message.ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: input,
			})
		}
	}

	return &provider.ChatResponse{
		Message: msg,
		Usage: provider.Usage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
		},
	}
}
