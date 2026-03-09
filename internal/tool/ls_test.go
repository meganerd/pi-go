package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLsTool_DirectoryListing(t *testing.T) {
	dir := t.TempDir()
	// Create files and a subdirectory
	if err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file2.go"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	tool := &LsTool{FS: &OSFileSystem{}}
	input, _ := json.Marshal(lsInput{Path: dir})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Output, "file1.txt") {
		t.Errorf("should list file1.txt, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "file2.go") {
		t.Errorf("should list file2.go, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "subdir/") {
		t.Errorf("should list subdir/ with trailing slash, got: %s", result.Output)
	}
}

func TestLsTool_NonexistentDirectory(t *testing.T) {
	tool := &LsTool{FS: &OSFileSystem{}}
	input, _ := json.Marshal(lsInput{Path: "/nonexistent/directory"})
	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}
