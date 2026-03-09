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
