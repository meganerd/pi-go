package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrepTool_PatternMatch(t *testing.T) {
	dir := t.TempDir()
	// Create test files
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello world\ngoodbye world\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("hello there\nnothing here\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := &GrepTool{}
	input, _ := json.Marshal(grepInput{Pattern: "hello", Path: dir})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Output, "hello") {
		t.Errorf("output should contain 'hello', got: %s", result.Output)
	}
	// Should match in both files
	if !strings.Contains(result.Output, "a.txt") || !strings.Contains(result.Output, "b.txt") {
		t.Errorf("should match in both files, got: %s", result.Output)
	}
}

func TestGrepTool_NoMatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello world\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := &GrepTool{}
	input, _ := json.Marshal(grepInput{Pattern: "zzzznothere", Path: dir})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Output != "" {
		t.Errorf("expected empty output for no match, got: %s", result.Output)
	}
}
