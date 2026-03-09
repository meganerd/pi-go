package provider

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/meganerd/pi-go/internal/message"
)

type retryMockProvider struct {
	calls    int
	failFor  int // number of calls to fail before succeeding
	err      error
	response *ChatResponse
}

func (m *retryMockProvider) Name() string { return "mock" }
func (m *retryMockProvider) Models() []Model { return nil }
func (m *retryMockProvider) Chat(_ context.Context, _ *ChatRequest) (*ChatResponse, error) {
	m.calls++
	if m.calls <= m.failFor {
		return nil, m.err
	}
	return m.response, nil
}

func TestRetryChat_SuccessFirstTry(t *testing.T) {
	resp := &ChatResponse{Message: message.Message{Content: "ok"}}
	mock := &retryMockProvider{response: resp}

	got, err := RetryChat(context.Background(), mock, &ChatRequest{}, DefaultRetryConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Message.Content != "ok" {
		t.Errorf("content = %q, want ok", got.Message.Content)
	}
	if mock.calls != 1 {
		t.Errorf("calls = %d, want 1", mock.calls)
	}
}

func TestRetryChat_RetryableError_ThenSuccess(t *testing.T) {
	resp := &ChatResponse{Message: message.Message{Content: "ok"}}
	mock := &retryMockProvider{
		failFor:  2,
		err:      fmt.Errorf("API error 429: rate limited"),
		response: resp,
	}

	cfg := RetryConfig{MaxRetries: 3, BaseDelay: 1 * time.Millisecond, MaxDelay: 10 * time.Millisecond}
	got, err := RetryChat(context.Background(), mock, &ChatRequest{}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Message.Content != "ok" {
		t.Errorf("content = %q, want ok", got.Message.Content)
	}
	if mock.calls != 3 {
		t.Errorf("calls = %d, want 3", mock.calls)
	}
}

func TestRetryChat_NonRetryableError(t *testing.T) {
	mock := &retryMockProvider{
		failFor: 10,
		err:     fmt.Errorf("invalid API key"),
	}

	cfg := RetryConfig{MaxRetries: 3, BaseDelay: 1 * time.Millisecond, MaxDelay: 10 * time.Millisecond}
	_, err := RetryChat(context.Background(), mock, &ChatRequest{}, cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if mock.calls != 1 {
		t.Errorf("calls = %d, want 1 (should not retry)", mock.calls)
	}
}

func TestRetryChat_ExhaustedRetries(t *testing.T) {
	mock := &retryMockProvider{
		failFor: 10,
		err:     fmt.Errorf("API error 503: service unavailable"),
	}

	cfg := RetryConfig{MaxRetries: 2, BaseDelay: 1 * time.Millisecond, MaxDelay: 10 * time.Millisecond}
	_, err := RetryChat(context.Background(), mock, &ChatRequest{}, cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if mock.calls != 3 { // initial + 2 retries
		t.Errorf("calls = %d, want 3", mock.calls)
	}
}

func TestRetryChat_ContextCancelled(t *testing.T) {
	mock := &retryMockProvider{
		failFor: 10,
		err:     fmt.Errorf("API error 429: rate limited"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	cfg := RetryConfig{MaxRetries: 3, BaseDelay: 1 * time.Second, MaxDelay: 10 * time.Second}
	_, err := RetryChat(ctx, mock, &ChatRequest{}, cfg)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{fmt.Errorf("invalid api key"), false},
		{fmt.Errorf("API error 429: rate limited"), true},
		{fmt.Errorf("API error 500: internal server error"), true},
		{fmt.Errorf("API error 502: bad gateway"), true},
		{fmt.Errorf("API error 503: service unavailable"), true},
		{fmt.Errorf("connection refused"), true},
		{fmt.Errorf("connection reset by peer"), true},
		{fmt.Errorf("unexpected EOF"), true},
		{fmt.Errorf("request timeout exceeded"), true},
		{fmt.Errorf("API error 401: unauthorized"), false},
	}
	for _, tt := range tests {
		got := isRetryable(tt.err)
		if got != tt.want {
			t.Errorf("isRetryable(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}
