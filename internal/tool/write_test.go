package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteTool_CreateInNewDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "deep", "file.txt")
	content := "hello world"

	tool := &WriteTool{FS: &OSFileSystem{}}
	input, _ := json.Marshal(writeInput{Path: path, Content: content})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file was created
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if string(data) != content {
		t.Errorf("content mismatch: got %q, want %q", string(data), content)
	}
	if result.Bytes != len(content) {
		t.Errorf("bytes mismatch: got %d, want %d", result.Bytes, len(content))
	}
}

func TestWriteTool_OverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")

	// Create initial file
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := &WriteTool{FS: &OSFileSystem{}}
	newContent := "overwritten"
	input, _ := json.Marshal(writeInput{Path: path, Content: newContent})
	_, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != newContent {
		t.Errorf("content mismatch: got %q, want %q", string(data), newContent)
	}
}
