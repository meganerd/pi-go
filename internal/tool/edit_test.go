package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEditTool_SuccessfulReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := &EditTool{FS: &OSFileSystem{}}
	input, _ := json.Marshal(editInput{
		Path:      path,
		OldString: "world",
		NewString: "gopher",
	})
	_, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello gopher" {
		t.Errorf("got %q, want %q", string(data), "hello gopher")
	}
}

func TestEditTool_PatternNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := &EditTool{FS: &OSFileSystem{}}
	input, _ := json.Marshal(editInput{
		Path:      path,
		OldString: "missing",
		NewString: "replacement",
	})
	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for pattern not found")
	}
}

func TestEditTool_NonUniqueMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.txt")
	if err := os.WriteFile(path, []byte("foo bar foo"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := &EditTool{FS: &OSFileSystem{}}
	input, _ := json.Marshal(editInput{
		Path:      path,
		OldString: "foo",
		NewString: "baz",
	})
	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for non-unique match")
	}
}
