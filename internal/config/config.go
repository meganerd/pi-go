// Package config defines configuration structures and loading for pi-go.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	DefaultModel            = "claude-sonnet-4-20250514"
	DefaultProvider         = "anthropic"
	DefaultMaxTokens        = 8192
	DefaultMaxContextTokens = 100000
)

// Config holds the full pi-go configuration, resolved from multiple layers.
type Config struct {
	// Model is the default LLM model ID.
	Model string `json:"model,omitempty"`

	// Provider is the default LLM provider name.
	Provider string `json:"provider,omitempty"`

	// SessionDir is the directory for session storage.
	SessionDir string `json:"session_dir,omitempty"`

	// SystemPrompt overrides the default system prompt.
	SystemPrompt string `json:"system_prompt,omitempty"`

	// MaxTokens is the max output tokens per LLM call.
	MaxTokens int `json:"max_tokens,omitempty"`

	// MaxContextTokens is the token threshold for context compaction.
	MaxContextTokens int `json:"max_context_tokens,omitempty"`

	// Tools lists the enabled tool names. Empty means all.
	Tools []string `json:"tools,omitempty"`

	// Et holds optional electrictown integration settings.
	Et *EtConfig `json:"et,omitempty"`
}

// EtConfig holds electrictown integration settings.
type EtConfig struct {
	Enabled    bool   `json:"enabled"`
	ConfigPath string `json:"config_path,omitempty"`
	OutputDir  string `json:"output_dir,omitempty"`
}

// DefaultSessionDir returns the default session storage directory.
func DefaultSessionDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "pi-go", "sessions")
}

// DefaultConfigPath returns the default config file path.
func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "pi-go", "config.json")
}

// Load reads configuration from the default config file, then overlays
// environment variables. CLI flags should be applied by the caller after Load.
func Load() *Config {
	cfg := &Config{}

	// Layer 1: config file
	path := DefaultConfigPath()
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, cfg)
	}

	// Layer 2: environment variables (override file)
	if v := os.Getenv("PI_GO_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("PI_GO_PROVIDER"); v != "" {
		cfg.Provider = v
	}

	// Layer 3: defaults for unset values
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	if cfg.Provider == "" {
		cfg.Provider = DefaultProvider
	}
	if cfg.SessionDir == "" {
		cfg.SessionDir = DefaultSessionDir()
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = DefaultMaxTokens
	}
	if cfg.MaxContextTokens == 0 {
		cfg.MaxContextTokens = DefaultMaxContextTokens
	}

	return cfg
}

// ApplyFlags overlays CLI flag values onto the config.
// Only non-zero values are applied.
func (c *Config) ApplyFlags(model, prov, sessionDir string) {
	if model != "" {
		c.Model = model
	}
	if prov != "" {
		c.Provider = prov
	}
	if sessionDir != "" {
		c.SessionDir = sessionDir
	}
}
