package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadTool_NormalRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "line one\nline two\nline three\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tool := &ReadTool{FS: &OSFileSystem{}}
	input, _ := json.Marshal(readInput{Path: path})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Truncated {
		t.Error("expected Truncated=false")
	}
	if !strings.Contains(result.Output, "line one") {
		t.Errorf("output should contain 'line one', got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "line three") {
		t.Errorf("output should contain 'line three', got: %s", result.Output)
	}
	// Check line numbers are present
	if !strings.Contains(result.Output, "1\t") {
		t.Error("output should contain line number 1")
	}
}

func TestReadTool_OffsetAndLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "a\nb\nc\nd\ne\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tool := &ReadTool{FS: &OSFileSystem{}}
	input, _ := json.Marshal(readInput{Path: path, Offset: 2, Limit: 2})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Truncated {
		t.Error("expected Truncated=true when limit applied")
	}
	if !strings.Contains(result.Output, "b") {
		t.Errorf("output should contain 'b' (line 2), got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "c") {
		t.Errorf("output should contain 'c' (line 3), got: %s", result.Output)
	}
	if strings.Contains(result.Output, "d") {
		t.Errorf("output should NOT contain 'd' (line 4), got: %s", result.Output)
	}
}

func TestReadTool_NonexistentFile(t *testing.T) {
	tool := &ReadTool{FS: &OSFileSystem{}}
	input, _ := json.Marshal(readInput{Path: "/nonexistent/path/file.txt"})
	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}
