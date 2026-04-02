package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscover_NoFiles(t *testing.T) {
	dir := t.TempDir()
	ctx, err := Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ctx.Files) != 0 {
		t.Errorf("expected no files, got %d", len(ctx.Files))
	}
}

func TestDiscover_ClaudeMD(t *testing.T) {
	dir := t.TempDir()
	content := "# Project Rules\nAlways use Go stdlib."
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(content), 0644)

	ctx, err := Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ctx.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(ctx.Files))
	}
	if ctx.Files[0].Content != content {
		t.Errorf("content = %q, want %q", ctx.Files[0].Content, content)
	}
}

func TestDiscover_PiGoMD(t *testing.T) {
	dir := t.TempDir()
	content := "Use gofmt."
	os.WriteFile(filepath.Join(dir, ".pi-go.md"), []byte(content), 0644)

	ctx, err := Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ctx.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(ctx.Files))
	}
	if !strings.Contains(ctx.Files[0].Path, ".pi-go.md") {
		t.Errorf("path = %q, expected .pi-go.md", ctx.Files[0].Path)
	}
}

func TestDiscover_PriorityOrder(t *testing.T) {
	dir := t.TempDir()
	// Both CLAUDE.md and .pi-go.md exist — CLAUDE.md should win
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("claude rules"), 0644)
	os.WriteFile(filepath.Join(dir, ".pi-go.md"), []byte("pigo rules"), 0644)

	ctx, err := Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ctx.Files) != 1 {
		t.Fatalf("expected 1 file (first match wins), got %d", len(ctx.Files))
	}
	if ctx.Files[0].Content != "claude rules" {
		t.Errorf("content = %q, want claude rules", ctx.Files[0].Content)
	}
}

func TestDiscover_ParentDirectory(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "subdir")
	os.MkdirAll(child, 0755)

	os.WriteFile(filepath.Join(parent, "CLAUDE.md"), []byte("parent rules"), 0644)
	os.WriteFile(filepath.Join(child, ".pi-go.md"), []byte("child rules"), 0644)

	ctx, err := Discover(child)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ctx.Files) != 2 {
		t.Fatalf("expected 2 files (child + parent), got %d", len(ctx.Files))
	}
	// Child should be first (most specific)
	if ctx.Files[0].Content != "child rules" {
		t.Errorf("first file content = %q, want child rules", ctx.Files[0].Content)
	}
	if ctx.Files[1].Content != "parent rules" {
		t.Errorf("second file content = %q, want parent rules", ctx.Files[1].Content)
	}
}

func TestDiscover_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("  \n  "), 0644)

	ctx, err := Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ctx.Files) != 0 {
		t.Errorf("empty/whitespace file should be skipped, got %d files", len(ctx.Files))
	}
}

func TestProjectContext_ForSystemPrompt_NoFiles(t *testing.T) {
	ctx := &ProjectContext{}
	if s := ctx.ForSystemPrompt(); s != "" {
		t.Errorf("empty context should produce empty string, got %q", s)
	}
}

func TestDiscoverSystemPromptFiles_SystemMD(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SYSTEM.md"), []byte("Custom system prompt."), 0644)

	sys, app := DiscoverSystemPromptFiles(dir)
	if sys != "Custom system prompt." {
		t.Errorf("system = %q, want 'Custom system prompt.'", sys)
	}
	if app != "" {
		t.Errorf("append should be empty, got %q", app)
	}
}

func TestDiscoverSystemPromptFiles_AppendMD(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "APPEND_SYSTEM.md"), []byte("Extra rules."), 0644)

	sys, app := DiscoverSystemPromptFiles(dir)
	if sys != "" {
		t.Errorf("system should be empty, got %q", sys)
	}
	if app != "Extra rules." {
		t.Errorf("append = %q, want 'Extra rules.'", app)
	}
}

func TestDiscoverSystemPromptFiles_Both(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SYSTEM.md"), []byte("Replace this."), 0644)
	os.WriteFile(filepath.Join(dir, "APPEND_SYSTEM.md"), []byte("Append this."), 0644)

	sys, app := DiscoverSystemPromptFiles(dir)
	if sys != "Replace this." {
		t.Errorf("system = %q, want 'Replace this.'", sys)
	}
	if app != "Append this." {
		t.Errorf("append = %q, want 'Append this.'", app)
	}
}

func TestDiscoverSystemPromptFiles_ParentDir(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "sub")
	os.MkdirAll(child, 0755)
	os.WriteFile(filepath.Join(parent, "SYSTEM.md"), []byte("Parent prompt."), 0644)

	sys, _ := DiscoverSystemPromptFiles(child)
	if sys != "Parent prompt." {
		t.Errorf("should find parent SYSTEM.md, got %q", sys)
	}
}

func TestDiscoverSystemPromptFiles_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SYSTEM.md"), []byte("  \n  "), 0644)

	sys, _ := DiscoverSystemPromptFiles(dir)
	if sys != "" {
		t.Errorf("empty SYSTEM.md should be ignored, got %q", sys)
	}
}

func TestProjectContext_ForSystemPrompt_WithFiles(t *testing.T) {
	ctx := &ProjectContext{
		Files: []ContextFile{
			{Path: "/project/CLAUDE.md", Content: "Always test first"},
		},
	}
	s := ctx.ForSystemPrompt()
	if !strings.Contains(s, "Project Context") {
		t.Errorf("should contain header: %s", s)
	}
	if !strings.Contains(s, "Always test first") {
		t.Errorf("should contain content: %s", s)
	}
	if !strings.Contains(s, "/project/CLAUDE.md") {
		t.Errorf("should contain path: %s", s)
	}
}
