package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindTool_GlobPattern(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"foo.go", "bar.go", "baz.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	tool := &FindTool{FS: &OSFileSystem{}}
	input, _ := json.Marshal(findInput{Pattern: "*.go", Path: dir})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Output, "foo.go") {
		t.Errorf("should find foo.go, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "bar.go") {
		t.Errorf("should find bar.go, got: %s", result.Output)
	}
	if strings.Contains(result.Output, "baz.txt") {
		t.Errorf("should NOT find baz.txt, got: %s", result.Output)
	}
}

func TestFindTool_ExcludeDirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "vendor")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "dep.go"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := &FindTool{FS: &OSFileSystem{}}
	input, _ := json.Marshal(findInput{Pattern: "*.go", Path: dir, Exclude: []string{"vendor"}})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Output, "main.go") {
		t.Errorf("should find main.go, got: %s", result.Output)
	}
	if strings.Contains(result.Output, "dep.go") {
		t.Errorf("should NOT find vendor/dep.go, got: %s", result.Output)
	}
}
