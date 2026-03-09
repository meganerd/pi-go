package tool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestBashTool_SuccessfulExecution(t *testing.T) {
	tool := &BashTool{}
	input, _ := json.Marshal(bashInput{Command: "echo hello"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Output, "hello") {
		t.Errorf("output should contain 'hello', got: %s", result.Output)
	}
}

func TestBashTool_Timeout(t *testing.T) {
	tool := &BashTool{}
	// Sleep for 5 seconds but timeout after 100ms
	input, _ := json.Marshal(bashInput{Command: "sleep 5", Timeout: 100})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Output, "timed out") {
		t.Errorf("output should indicate timeout, got: %s", result.Output)
	}
	if !result.Truncated {
		t.Error("expected Truncated=true on timeout")
	}
}
