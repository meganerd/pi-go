// Package config defines configuration structures for pi-go.
package config

// Config holds the full pi-go configuration, resolved from multiple layers.
type Config struct {
	// Model is the default LLM model ID.
	Model string `json:"model,omitempty" yaml:"model,omitempty"`

	// Provider is the default LLM provider name.
	Provider string `json:"provider,omitempty" yaml:"provider,omitempty"`

	// SessionDir is the directory for session storage.
	SessionDir string `json:"session_dir,omitempty" yaml:"session_dir,omitempty"`

	// Tools lists the enabled tool names. Empty means all.
	Tools []string `json:"tools,omitempty" yaml:"tools,omitempty"`

	// Et holds optional electrictown integration settings.
	Et *EtConfig `json:"et,omitempty" yaml:"et,omitempty"`
}

// EtConfig holds electrictown integration settings.
type EtConfig struct {
	// Enabled controls whether et integration is active.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// ConfigPath is the path to the electrictown.yaml config file.
	ConfigPath string `json:"config_path,omitempty" yaml:"config_path,omitempty"`

	// OutputDir is the default output directory for et results.
	OutputDir string `json:"output_dir,omitempty" yaml:"output_dir,omitempty"`
}
