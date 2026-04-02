package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompleteFilePath_CurrentDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0600)
	os.WriteFile(filepath.Join(dir, "main_test.go"), []byte("package main"), 0600)
	os.MkdirAll(filepath.Join(dir, "internal"), 0755)

	matches := CompleteFilePath(filepath.Join(dir, "main"))
	if len(matches) < 2 {
		t.Errorf("expected at least 2 matches for 'main', got %d: %v", len(matches), matches)
	}
}

func TestCompleteFilePath_DirPrefix(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(sub, "file.txt"), []byte("data"), 0600)

	matches := CompleteFilePath(sub + "/")
	if len(matches) != 1 {
		t.Errorf("expected 1 match, got %d: %v", len(matches), matches)
	}
}

func TestCompleteFilePath_NoMatches(t *testing.T) {
	matches := CompleteFilePath("/nonexistent/path/zzz")
	if len(matches) != 0 {
		t.Errorf("expected no matches, got %d", len(matches))
	}
}

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		s, q string
		want bool
	}{
		{"internal/config/config.go", "cfg", true},
		{"internal/config/config.go", "icfg", true},
		{"main.go", "mg", true},
		{"main.go", "xyz", false},
		{"README.md", "rm", true},
		{"", "a", false},
		{"abc", "", true},
	}
	for _, tt := range tests {
		if got := fuzzyMatch(tt.s, tt.q); got != tt.want {
			t.Errorf("fuzzyMatch(%q, %q) = %v, want %v", tt.s, tt.q, got, tt.want)
		}
	}
}

func TestFuzzyFindFiles_Basic(t *testing.T) {
	dir := t.TempDir()
	// Create some files
	os.WriteFile(filepath.Join(dir, "config.go"), []byte("package config"), 0600)
	os.WriteFile(filepath.Join(dir, "config_test.go"), []byte("package config"), 0600)
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "sub", "handler.go"), []byte("package sub"), 0600)

	// Change to temp dir for the test
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(dir)

	matches := FuzzyFindFiles("cfg", 10)
	if len(matches) < 2 {
		t.Errorf("expected at least 2 matches for 'cfg', got %d: %v", len(matches), matches)
	}
}

func TestFuzzyFindFiles_SkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git", "objects"), 0755)
	os.WriteFile(filepath.Join(dir, ".git", "objects", "test.go"), []byte("data"), 0600)
	os.WriteFile(filepath.Join(dir, "visible.go"), []byte("data"), 0600)

	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(dir)

	matches := FuzzyFindFiles("go", 100)
	for _, m := range matches {
		if filepath.Base(filepath.Dir(m)) == ".git" || filepath.Base(m) == "test.go" {
			t.Errorf("should skip .git dir, found: %s", m)
		}
	}
}

func TestFuzzyFindFiles_MaxResults(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 20; i++ {
		os.WriteFile(filepath.Join(dir, filepath.Base(dir)+string(rune('a'+i))+".go"), []byte("data"), 0600)
	}

	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(dir)

	matches := FuzzyFindFiles("go", 5)
	if len(matches) > 5 {
		t.Errorf("expected max 5 matches, got %d", len(matches))
	}
}
