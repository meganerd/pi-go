package provider

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

// RetryConfig controls retry behavior for transient provider errors.
type RetryConfig struct {
	MaxRetries int           // Maximum number of retries (0 = no retries)
	BaseDelay  time.Duration // Initial delay between retries
	MaxDelay   time.Duration // Maximum delay between retries
}

// DefaultRetryConfig returns sensible defaults for API retries.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
		MaxDelay:   30 * time.Second,
	}
}

// isRetryable returns true if the error is likely transient and worth retrying.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// Rate limits (429) and server errors (500, 502, 503, 529)
	for _, code := range []string{"429", "500", "502", "503", "529"} {
		if strings.Contains(msg, fmt.Sprintf("error %s", code)) {
			return true
		}
	}
	// Connection errors
	for _, hint := range []string{"connection refused", "connection reset", "eof", "timeout"} {
		if strings.Contains(strings.ToLower(msg), hint) {
			return true
		}
	}
	return false
}

// RetryChat wraps a provider's Chat call with exponential backoff retry.
func RetryChat(ctx context.Context, p Provider, req *ChatRequest, cfg RetryConfig) (*ChatResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		resp, err := p.Chat(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		if !isRetryable(err) || attempt == cfg.MaxRetries {
			return nil, err
		}

		delay := time.Duration(float64(cfg.BaseDelay) * math.Pow(2, float64(attempt)))
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, lastErr
}
