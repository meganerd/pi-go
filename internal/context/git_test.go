package context

import (
	"os"
	"strings"
	"testing"
)

func TestGitContext_InGitRepo(t *testing.T) {
	// Run from the pi-go repo itself
	cwd, _ := os.Getwd()
	result := GitContext(cwd)
	if result == "" {
		t.Skip("not running inside a git repo")
	}
	if !strings.Contains(result, "Repository:") {
		t.Errorf("should contain Repository:, got: %s", result)
	}
	if !strings.Contains(result, "Branch:") {
		t.Errorf("should contain Branch:, got: %s", result)
	}
}

func TestGitContext_NotGitRepo(t *testing.T) {
	dir := t.TempDir()
	result := GitContext(dir)
	if result != "" {
		t.Errorf("should return empty for non-git dir, got: %s", result)
	}
}

func TestGitContext_ContainsBranch(t *testing.T) {
	cwd, _ := os.Getwd()
	result := GitContext(cwd)
	if result == "" {
		t.Skip("not running inside a git repo")
	}
	if !strings.Contains(result, "Branch:") {
		t.Errorf("should contain Branch:, got: %s", result)
	}
}

func TestGitContext_ContainsCommits(t *testing.T) {
	cwd, _ := os.Getwd()
	result := GitContext(cwd)
	if result == "" {
		t.Skip("not running inside a git repo")
	}
	if !strings.Contains(result, "Recent commits:") {
		t.Errorf("should contain Recent commits:, got: %s", result)
	}
}
