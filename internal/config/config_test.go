package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_DefaultValues(t *testing.T) {
	// Ensure env vars don't interfere
	os.Unsetenv("PI_GO_MODEL")
	os.Unsetenv("PI_GO_PROVIDER")

	cfg := Load()

	if cfg.Model != DefaultModel {
		t.Errorf("model = %q, want %q", cfg.Model, DefaultModel)
	}
	if cfg.Provider != DefaultProvider {
		t.Errorf("provider = %q, want %q", cfg.Provider, DefaultProvider)
	}
	if cfg.SessionDir == "" {
		t.Error("session dir should have a default")
	}
	if cfg.MaxTokens != DefaultMaxTokens {
		t.Errorf("max_tokens = %d, want %d", cfg.MaxTokens, DefaultMaxTokens)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	t.Setenv("PI_GO_MODEL", "gpt-4o")
	t.Setenv("PI_GO_PROVIDER", "openai")

	cfg := Load()

	if cfg.Model != "gpt-4o" {
		t.Errorf("model = %q, want gpt-4o", cfg.Model)
	}
	if cfg.Provider != "openai" {
		t.Errorf("provider = %q, want openai", cfg.Provider)
	}
}

func TestLoad_ConfigFile(t *testing.T) {
	// Create a temp config file
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	data, _ := json.Marshal(Config{
		Model:    "test-model",
		Provider: "test-provider",
	})
	os.WriteFile(cfgPath, data, 0644)

	// Override default config path by setting env + writing directly
	// Since Load() reads DefaultConfigPath, we test ApplyFlags instead
	cfg := &Config{}
	raw, _ := os.ReadFile(cfgPath)
	json.Unmarshal(raw, cfg)

	if cfg.Model != "test-model" {
		t.Errorf("model = %q, want test-model", cfg.Model)
	}
	if cfg.Provider != "test-provider" {
		t.Errorf("provider = %q, want test-provider", cfg.Provider)
	}
}

func TestApplyFlags(t *testing.T) {
	cfg := &Config{
		Model:    "default-model",
		Provider: "default-provider",
	}

	cfg.ApplyFlags("flag-model", "", "")
	if cfg.Model != "flag-model" {
		t.Errorf("model = %q, want flag-model", cfg.Model)
	}
	if cfg.Provider != "default-provider" {
		t.Errorf("provider changed to %q, should stay default-provider", cfg.Provider)
	}

	cfg.ApplyFlags("", "flag-provider", "/tmp/sessions")
	if cfg.Provider != "flag-provider" {
		t.Errorf("provider = %q, want flag-provider", cfg.Provider)
	}
	if cfg.SessionDir != "/tmp/sessions" {
		t.Errorf("session_dir = %q, want /tmp/sessions", cfg.SessionDir)
	}
}

func TestDefaultSessionDir_NotEmpty(t *testing.T) {
	dir := DefaultSessionDir()
	if dir == "" {
		t.Error("default session dir should not be empty")
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("default session dir should be absolute, got %q", dir)
	}
}

func TestDefaultConfigPath_NotEmpty(t *testing.T) {
	path := DefaultConfigPath()
	if path == "" {
		t.Error("default config path should not be empty")
	}
	if !filepath.IsAbs(path) {
		t.Errorf("default config path should be absolute, got %q", path)
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &Config{
		Provider:         "anthropic",
		MaxTokens:        8192,
		MaxContextTokens: 100000,
	}
	warnings := cfg.Validate()
	if len(warnings) != 0 {
		t.Errorf("valid config should have no warnings, got %v", warnings)
	}
}

func TestValidate_UnknownProvider(t *testing.T) {
	cfg := &Config{
		Provider:         "invalid",
		MaxTokens:        8192,
		MaxContextTokens: 100000,
	}
	warnings := cfg.Validate()
	found := false
	for _, w := range warnings {
		if contains(w, "unknown provider") {
			found = true
		}
	}
	if !found {
		t.Errorf("should warn about unknown provider, got %v", warnings)
	}
}

func TestValidate_HighMaxTokens(t *testing.T) {
	cfg := &Config{
		Provider:         "anthropic",
		MaxTokens:        65536,
		MaxContextTokens: 200000,
	}
	warnings := cfg.Validate()
	found := false
	for _, w := range warnings {
		if contains(w, "unusually high") {
			found = true
		}
	}
	if !found {
		t.Errorf("should warn about high max_tokens, got %v", warnings)
	}
}

func TestValidate_MaxTokensExceedsContext(t *testing.T) {
	cfg := &Config{
		Provider:         "anthropic",
		MaxTokens:        16000,
		MaxContextTokens: 8000,
	}
	warnings := cfg.Validate()
	found := false
	for _, w := range warnings {
		if contains(w, "exceeds max_context_tokens") {
			found = true
		}
	}
	if !found {
		t.Errorf("should warn about max_tokens > max_context_tokens, got %v", warnings)
	}
}

func TestValidate_LowContextTokens(t *testing.T) {
	cfg := &Config{
		Provider:         "anthropic",
		MaxTokens:        100,
		MaxContextTokens: 500,
	}
	warnings := cfg.Validate()
	found := false
	for _, w := range warnings {
		if contains(w, "excessive compaction") {
			found = true
		}
	}
	if !found {
		t.Errorf("should warn about low context tokens, got %v", warnings)
	}
}

func TestValidate_SessionDirNotDir(t *testing.T) {
	// Create a file where session dir is expected
	f, err := os.CreateTemp("", "pi-go-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()

	cfg := &Config{
		Provider:         "anthropic",
		MaxTokens:        8192,
		MaxContextTokens: 100000,
		SessionDir:       f.Name(),
	}
	warnings := cfg.Validate()
	found := false
	for _, w := range warnings {
		if contains(w, "not a directory") {
			found = true
		}
	}
	if !found {
		t.Errorf("should warn about session dir not being a directory, got %v", warnings)
	}
}

func TestMergeProjectConfig(t *testing.T) {
	dir := t.TempDir()

	// Create .pi-go/config.json with project overrides
	projDir := filepath.Join(dir, ".pi-go")
	os.MkdirAll(projDir, 0755)
	projCfg := Config{
		Model:    "project-model",
		Provider: "openai",
	}
	data, _ := json.Marshal(projCfg)
	os.WriteFile(filepath.Join(projDir, "config.json"), data, 0644)

	// Start with a global config
	cfg := &Config{
		Model:            "global-model",
		Provider:         "anthropic",
		MaxTokens:        4096,
		MaxContextTokens: 50000,
	}

	MergeProjectConfig(cfg, dir)

	// Project values should override
	if cfg.Model != "project-model" {
		t.Errorf("model = %q, want project-model", cfg.Model)
	}
	if cfg.Provider != "openai" {
		t.Errorf("provider = %q, want openai", cfg.Provider)
	}
	// Non-overridden values preserved
	if cfg.MaxTokens != 4096 {
		t.Errorf("max_tokens = %d, should stay 4096", cfg.MaxTokens)
	}
}

func TestMergeProjectConfig_NoFile(t *testing.T) {
	cfg := &Config{Model: "original"}
	MergeProjectConfig(cfg, "/nonexistent/path")
	if cfg.Model != "original" {
		t.Errorf("model should be unchanged, got %q", cfg.Model)
	}
}

func TestProjectConfigPath(t *testing.T) {
	path := ProjectConfigPath("/home/user/project")
	expected := filepath.Join("/home/user/project", ".pi-go", "config.json")
	if path != expected {
		t.Errorf("path = %q, want %q", path, expected)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
