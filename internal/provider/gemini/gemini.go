// Package gemini implements the Google Gemini API provider using the
// generateContent REST endpoint. It supports both synchronous and streaming
// chat, tool/function calling, and system instructions.
package gemini

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

const defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"

// Gemini implements provider.Provider and provider.StreamProvider for the
// Google Gemini API.
type Gemini struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// New creates a new Gemini provider. If baseURL is empty, the default Google
// API endpoint is used.
func New(apiKey, baseURL string) *Gemini {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Gemini{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

func (g *Gemini) Name() string { return "gemini" }

func (g *Gemini) Models() []provider.Model {
	return []provider.Model{
		{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", Provider: "gemini", ContextWindow: 1048576},
		{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Provider: "gemini", ContextWindow: 1048576},
	}
}

// --- Wire types (Gemini REST API) ---

type apiRequest struct {
	Contents          []apiContent         `json:"contents"`
	Tools             []apiTool            `json:"tools,omitempty"`
	SystemInstruction *apiContent          `json:"systemInstruction,omitempty"`
	GenerationConfig  *apiGenerationConfig `json:"generationConfig,omitempty"`
}

type apiContent struct {
	Role  string    `json:"role,omitempty"`
	Parts []apiPart `json:"parts"`
}

// apiPart is a union type — only one field should be set.
type apiPart struct {
	Text             string            `json:"text,omitempty"`
	FunctionCall     *apiFunctionCall  `json:"functionCall,omitempty"`
	FunctionResponse *apiFunctionResp  `json:"functionResponse,omitempty"`
}

type apiFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args,omitempty"`
}

type apiFunctionResp struct {
	Name     string                 `json:"name"`
	Response map[string]interface{} `json:"response"`
}

type apiTool struct {
	FunctionDeclarations []apiFunctionDecl `json:"functionDeclarations"`
}

type apiFunctionDecl struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type apiGenerationConfig struct {
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
}

type apiResponse struct {
	Candidates    []apiCandidate `json:"candidates"`
	UsageMetadata *apiUsage      `json:"usageMetadata,omitempty"`
}

type apiCandidate struct {
	Content       apiContent `json:"content"`
	FinishReason  string     `json:"finishReason"`
}

type apiUsage struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
}

// Chat sends a synchronous generateContent request.
func (g *Gemini) Chat(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	apiReq := g.buildRequest(req)
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", g.baseURL, req.Model, g.apiKey)

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini API error %d: %s", httpResp.StatusCode, string(respBody))
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return g.parseResponse(&apiResp), nil
}

// StreamChat sends a streaming generateContent request using SSE.
func (g *Gemini) StreamChat(ctx context.Context, req *provider.ChatRequest) (<-chan provider.StreamEvent, error) {
	apiReq := g.buildRequest(req)
	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse&key=%s", g.baseURL, req.Model, g.apiKey)

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()
		return nil, fmt.Errorf("gemini API error %d: %s", httpResp.StatusCode, string(respBody))
	}

	out := make(chan provider.StreamEvent, 32)
	go func() {
		defer close(out)
		defer httpResp.Body.Close()

		for sse := range provider.ParseSSE(httpResp.Body) {
			events := g.parseStreamChunk(sse.Data)
			for _, e := range events {
				select {
				case out <- e:
				case <-ctx.Done():
					out <- provider.StreamEvent{Type: "error", Error: ctx.Err()}
					return
				}
			}
		}
		out <- provider.StreamEvent{Type: "done"}
	}()

	return out, nil
}

// parseStreamChunk parses a single SSE data payload from Gemini streaming.
func (g *Gemini) parseStreamChunk(data string) []provider.StreamEvent {
	var resp apiResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		return nil
	}

	var events []provider.StreamEvent
	for _, cand := range resp.Candidates {
		for _, part := range cand.Content.Parts {
			if part.Text != "" {
				events = append(events, provider.StreamEvent{
					Type:    "text",
					Content: part.Text,
				})
			}
			if part.FunctionCall != nil {
				// Emit start + delta (full args) + end for each function call.
				events = append(events, provider.StreamEvent{
					Type:     "tool_use_start",
					ToolID:   "gemini_" + part.FunctionCall.Name,
					ToolName: part.FunctionCall.Name,
				})
				args, _ := json.Marshal(part.FunctionCall.Args)
				events = append(events, provider.StreamEvent{
					Type:    "tool_use_delta",
					Content: string(args),
				})
				events = append(events, provider.StreamEvent{Type: "tool_use_end"})
			}
		}
	}
	return events
}

// buildRequest converts a provider.ChatRequest to the Gemini API format.
func (g *Gemini) buildRequest(req *provider.ChatRequest) *apiRequest {
	apiReq := &apiRequest{}

	// System instruction.
	if req.SystemPrompt != "" {
		apiReq.SystemInstruction = &apiContent{
			Parts: []apiPart{{Text: req.SystemPrompt}},
		}
	}

	// Generation config.
	if req.MaxTokens > 0 || req.Temperature > 0 {
		apiReq.GenerationConfig = &apiGenerationConfig{
			MaxOutputTokens: req.MaxTokens,
			Temperature:     req.Temperature,
		}
	}

	// Convert messages to contents.
	apiReq.Contents = convertMessages(req.Messages)

	// Convert tools to function declarations.
	if len(req.Tools) > 0 {
		var decls []apiFunctionDecl
		for _, t := range req.Tools {
			decls = append(decls, apiFunctionDecl{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Schema,
			})
		}
		apiReq.Tools = []apiTool{{FunctionDeclarations: decls}}
	}

	return apiReq
}

// convertMessages maps internal messages to Gemini content format.
// Gemini uses "user" and "model" roles, with function calls/responses as parts.
func convertMessages(msgs []message.Message) []apiContent {
	var contents []apiContent

	for _, msg := range msgs {
		switch msg.Role {
		case message.RoleUser:
			contents = append(contents, apiContent{
				Role:  "user",
				Parts: []apiPart{{Text: msg.Content}},
			})

		case message.RoleAssistant:
			c := apiContent{Role: "model"}
			if msg.Content != "" {
				c.Parts = append(c.Parts, apiPart{Text: msg.Content})
			}
			for _, tc := range msg.ToolCalls {
				var args map[string]interface{}
				json.Unmarshal(tc.Input, &args)
				c.Parts = append(c.Parts, apiPart{
					FunctionCall: &apiFunctionCall{
						Name: tc.Name,
						Args: args,
					},
				})
			}
			if len(c.Parts) > 0 {
				contents = append(contents, c)
			}

		case message.RoleTool:
			if msg.ToolResult != nil {
				// Parse the tool call ID to get the function name.
				// We use the format "gemini_<name>" for IDs.
				name := msg.ToolResult.ToolCallID
				if len(name) > 7 && name[:7] == "gemini_" {
					name = name[7:]
				}
				respData := map[string]interface{}{
					"content": msg.ToolResult.Content,
				}
				if msg.ToolResult.IsError {
					respData["error"] = msg.ToolResult.Content
				}
				contents = append(contents, apiContent{
					Role: "user",
					Parts: []apiPart{{
						FunctionResponse: &apiFunctionResp{
							Name:     name,
							Response: respData,
						},
					}},
				})
			}

		case message.RoleSystem:
			// System messages are handled via SystemInstruction, not contents.
			// If they appear in the message list, convert to user message.
			contents = append(contents, apiContent{
				Role:  "user",
				Parts: []apiPart{{Text: msg.Content}},
			})
		}
	}

	return contents
}

// parseResponse converts a Gemini API response to the internal format.
func (g *Gemini) parseResponse(resp *apiResponse) *provider.ChatResponse {
	if len(resp.Candidates) == 0 {
		return &provider.ChatResponse{
			Message: message.Message{Role: message.RoleAssistant},
		}
	}

	cand := resp.Candidates[0]
	msg := message.Message{Role: message.RoleAssistant}

	for _, part := range cand.Content.Parts {
		if part.Text != "" {
			msg.Content += part.Text
		}
		if part.FunctionCall != nil {
			args, _ := json.Marshal(part.FunctionCall.Args)
			msg.ToolCalls = append(msg.ToolCalls, message.ToolCall{
				ID:    "gemini_" + part.FunctionCall.Name,
				Name:  part.FunctionCall.Name,
				Input: json.RawMessage(args),
			})
		}
	}

	chatResp := &provider.ChatResponse{
		Message:    msg,
		StopReason: cand.FinishReason,
	}

	if resp.UsageMetadata != nil {
		chatResp.Usage = provider.Usage{
			InputTokens:  resp.UsageMetadata.PromptTokenCount,
			OutputTokens: resp.UsageMetadata.CandidatesTokenCount,
		}
	}

	return chatResp
}

// Compile-time interface checks.
var (
	_ provider.Provider       = (*Gemini)(nil)
	_ provider.StreamProvider = (*Gemini)(nil)
)
