// Package config defines configuration structures and loading for pi-go.
package config

import (
	"encoding/json"
	"fmt"
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

// ProjectConfigPath returns the project-local config file path for a directory.
// Convention: .pi-go/config.json in the given directory.
func ProjectConfigPath(dir string) string {
	return filepath.Join(dir, ".pi-go", "config.json")
}

// Load reads configuration from the global config file, overlays the
// project-local config, then applies environment variables. CLI flags
// should be applied by the caller after Load.
func Load() *Config {
	cfg := &Config{}

	// Layer 1: global config file
	path := DefaultConfigPath()
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, cfg)
	}

	// Layer 2: project-local config (overrides global)
	cwd, err := os.Getwd()
	if err == nil {
		MergeProjectConfig(cfg, cwd)
	}

	// Layer 3: environment variables (override both)
	if v := os.Getenv("PI_GO_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("PI_GO_PROVIDER"); v != "" {
		cfg.Provider = v
	}

	// Layer 4: defaults for unset values
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

// MergeProjectConfig reads the project-local config from dir and merges
// non-zero values into cfg. Project config wins over global config.
func MergeProjectConfig(cfg *Config, dir string) {
	path := ProjectConfigPath(dir)
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var proj Config
	if err := json.Unmarshal(data, &proj); err != nil {
		return
	}
	// Merge: project values override global when set.
	if proj.Model != "" {
		cfg.Model = proj.Model
	}
	if proj.Provider != "" {
		cfg.Provider = proj.Provider
	}
	if proj.SessionDir != "" {
		cfg.SessionDir = proj.SessionDir
	}
	if proj.SystemPrompt != "" {
		cfg.SystemPrompt = proj.SystemPrompt
	}
	if proj.MaxTokens > 0 {
		cfg.MaxTokens = proj.MaxTokens
	}
	if proj.MaxContextTokens > 0 {
		cfg.MaxContextTokens = proj.MaxContextTokens
	}
	if len(proj.Tools) > 0 {
		cfg.Tools = proj.Tools
	}
	if proj.Et != nil {
		cfg.Et = proj.Et
	}
}

// Validate checks the configuration for potential issues and returns warnings.
// It does not return errors for values that would prevent startup — those are
// handled at initialization time. Warnings are informational.
func (c *Config) Validate() []string {
	var warnings []string

	// Provider validation
	validProviders := map[string]bool{"anthropic": true, "openai": true, "openrouter": true, "gemini": true, "zai": true}
	if !validProviders[c.Provider] {
		warnings = append(warnings, fmt.Sprintf("unknown provider %q — valid: anthropic, openai, openrouter, gemini, zai", c.Provider))
	}

	// Token limits
	if c.MaxTokens < 1 {
		warnings = append(warnings, "max_tokens should be at least 1")
	}
	if c.MaxTokens > 32768 {
		warnings = append(warnings, fmt.Sprintf("max_tokens=%d is unusually high — most models cap at 8192-16384", c.MaxTokens))
	}
	if c.MaxContextTokens < 1000 {
		warnings = append(warnings, "max_context_tokens below 1000 may cause excessive compaction")
	}
	if c.MaxTokens > c.MaxContextTokens {
		warnings = append(warnings, "max_tokens exceeds max_context_tokens — output may be truncated")
	}

	// Session directory
	if c.SessionDir != "" {
		if info, err := os.Stat(c.SessionDir); err == nil && !info.IsDir() {
			warnings = append(warnings, fmt.Sprintf("session_dir %q exists but is not a directory", c.SessionDir))
		}
	}

	// Et config
	if c.Et != nil && c.Et.Enabled {
		if c.Et.ConfigPath != "" {
			if _, err := os.Stat(c.Et.ConfigPath); err != nil {
				warnings = append(warnings, fmt.Sprintf("et config_path %q not found", c.Et.ConfigPath))
			}
		}
	}

	return warnings
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
