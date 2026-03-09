package et

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewCLIDelegator_Defaults(t *testing.T) {
	d := NewCLIDelegator("", "", "")
	if d.binary != "et" {
		t.Errorf("binary = %q, want et", d.binary)
	}
	if d.outputDir != "/tmp/pi-go-et" {
		t.Errorf("outputDir = %q, want /tmp/pi-go-et", d.outputDir)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, "electrictown.yaml")
	if d.configPath != want {
		t.Errorf("configPath = %q, want %q", d.configPath, want)
	}
}

func TestNewCLIDelegator_CustomValues(t *testing.T) {
	d := NewCLIDelegator("/usr/bin/et", "/etc/et.yaml", "/tmp/custom")
	if d.binary != "/usr/bin/et" {
		t.Errorf("binary = %q", d.binary)
	}
	if d.configPath != "/etc/et.yaml" {
		t.Errorf("configPath = %q", d.configPath)
	}
	if d.outputDir != "/tmp/custom" {
		t.Errorf("outputDir = %q", d.outputDir)
	}
}

func TestCLIDelegator_Available_NoBinary(t *testing.T) {
	d := NewCLIDelegator("/nonexistent/binary/et", "", "")
	if d.Available() {
		t.Error("should not be available when binary doesn't exist")
	}
}

func TestCLIDelegator_Available_NoConfig(t *testing.T) {
	d := NewCLIDelegator("echo", "/nonexistent/config.yaml", "")
	if d.Available() {
		t.Error("should not be available when config doesn't exist")
	}
}

func TestFindOutputFiles(t *testing.T) {
	dir := t.TempDir()
	// Create some test files
	os.WriteFile(filepath.Join(dir, "file1.go"), []byte("package main"), 0644)
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "sub", "file2.go"), []byte("package sub"), 0644)

	files := findOutputFiles(dir)
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d: %v", len(files), files)
	}
}

func TestFindOutputFiles_Empty(t *testing.T) {
	dir := t.TempDir()
	files := findOutputFiles(dir)
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}
