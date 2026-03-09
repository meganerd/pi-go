// pi-go is a Go rewrite of the pi coding agent — an LLM-powered coding assistant.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/meganerd/pi-go/internal/agent"
	"github.com/meganerd/pi-go/internal/compact"
	"github.com/meganerd/pi-go/internal/config"
	"github.com/meganerd/pi-go/internal/et"
	"github.com/meganerd/pi-go/internal/provider"
	"github.com/meganerd/pi-go/internal/provider/anthropic"
	"github.com/meganerd/pi-go/internal/provider/openai"
	"github.com/meganerd/pi-go/internal/session"
	"github.com/meganerd/pi-go/internal/tool"
	"github.com/meganerd/pi-go/internal/tui"
)

var version = "dev"

func main() {
	var (
		showVersion bool
		model       string
		providerArg string
		sessionDir  string
		resume      bool
	)

	flag.BoolVar(&showVersion, "version", false, "Print version and exit")
	flag.StringVar(&model, "model", "", "LLM model to use")
	flag.StringVar(&providerArg, "provider", "", "LLM provider (anthropic, openai, openrouter)")
	flag.StringVar(&sessionDir, "session-dir", "", "Session storage directory")
	flag.BoolVar(&resume, "resume", false, "Resume last session for current directory")
	flag.Parse()

	if showVersion {
		fmt.Printf("pi-go %s\n", version)
		os.Exit(0)
	}

	// Load and merge configuration
	cfg := config.Load()
	cfg.ApplyFlags(model, providerArg, sessionDir)

	// Initialize provider
	prov, err := initProvider(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Initialize tools
	fs := &tool.OSFileSystem{}
	tools := []tool.Tool{
		&tool.ReadTool{FS: fs},
		&tool.WriteTool{FS: fs},
		&tool.EditTool{FS: fs},
		&tool.BashTool{},
		&tool.GrepTool{},
		&tool.FindTool{FS: fs},
		&tool.LsTool{FS: fs},
	}

	// Add et_delegate tool if electrictown is available
	etDelegator := initEtDelegator(cfg)
	if etDelegator != nil && etDelegator.Available() {
		tools = append(tools, &tool.EtDelegateTool{Delegator: etDelegator})
		fmt.Fprintln(os.Stderr, "et: electrictown integration active")
	}

	// Initialize session
	sess, err := initSession(cfg, resume)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: session unavailable: %v\n", err)
	}

	// Build compactor for context management
	compactor := compact.New(prov, cfg.Model, cfg.MaxContextTokens, 0)

	// Build agent loop with tool activity callback, compaction, and streaming
	loop := agent.New(prov, tools).
		WithCompactor(compactor).
		WithToolCallback(func(name string, isResult bool, output string, isError bool) {
			if !isResult {
				fmt.Fprintf(os.Stderr, "\n[tool: %s]\n", name)
			} else if isError {
				fmt.Fprintf(os.Stderr, "[%s error: %s]\n", name, truncate(output, 200))
			} else {
				fmt.Fprintf(os.Stderr, "[%s done: %d bytes]\n", name, len(output))
			}
		}).
		WithStreamCallback(func(text string) {
			fmt.Print(text)
		})
	if sess != nil {
		loop = loop.WithSession(sess)
	}

	// Build system prompt
	systemPrompt := cfg.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = tui.DefaultSystemPrompt(tools)
	}

	// Run TUI
	opts := []tui.Option{
		tui.WithModel(cfg.Model),
		tui.WithSystemPrompt(systemPrompt),
	}
	if provider.CanStream(prov) {
		opts = append(opts, tui.WithStreaming())
	}
	if sess != nil {
		opts = append(opts, tui.WithSession(sess))
	}

	ui := tui.New(loop, opts...)
	if err := ui.Run(context.Background()); err != nil {
		if sess != nil {
			sess.Close()
		}
		os.Exit(0) // /exit returns io.EOF — clean exit
	}

	if sess != nil {
		sess.Close()
	}
}

func initProvider(cfg *config.Config) (provider.Provider, error) {
	reg := provider.NewRegistry()
	reg.Register("anthropic", func(apiKey, baseURL string) provider.Provider {
		return anthropic.New(apiKey, baseURL)
	})
	reg.Register("openai", func(apiKey, baseURL string) provider.Provider {
		return openai.New(apiKey, baseURL)
	})
	reg.Register("openrouter", func(apiKey, _ string) provider.Provider {
		return openai.NewOpenRouter(apiKey)
	})

	// Resolve API key from environment
	apiKey := ""
	switch cfg.Provider {
	case "anthropic":
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable not set. Set it with: export ANTHROPIC_API_KEY=your-key")
		}
	case "openai":
		apiKey = os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY environment variable not set. Set it with: export OPENAI_API_KEY=your-key")
		}
	case "openrouter":
		apiKey = os.Getenv("OPENROUTER_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("OPENROUTER_API_KEY environment variable not set. Set it with: export OPENROUTER_API_KEY=your-key")
		}
	default:
		return nil, fmt.Errorf("unknown provider %q. Available: anthropic, openai, openrouter", cfg.Provider)
	}

	prov := reg.Get(cfg.Provider, apiKey, "")
	if prov == nil {
		return nil, fmt.Errorf("failed to initialize provider %q", cfg.Provider)
	}
	return prov, nil
}

func initSession(cfg *config.Config, resume bool) (session.Store, error) {
	if err := os.MkdirAll(cfg.SessionDir, 0755); err != nil {
		return nil, fmt.Errorf("create session directory: %w", err)
	}

	mgr := session.NewManager(cfg.SessionDir)

	if resume {
		// Find last session for current directory
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
		sessions, err := mgr.ForPath(cwd)
		if err != nil || len(sessions) == 0 {
			fmt.Fprintln(os.Stderr, "No previous session found, starting new session")
		} else {
			// Open the most recent session
			store, err := mgr.Open(sessions[0].ID)
			if err != nil {
				return nil, fmt.Errorf("open session: %w", err)
			}
			return store, nil
		}
	}

	// Create new session
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	store, _, err := mgr.Create(cwd)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return store, nil
}

func initEtDelegator(cfg *config.Config) *et.CLIDelegator {
	var configPath, outputDir string
	if cfg.Et != nil {
		if !cfg.Et.Enabled {
			return nil
		}
		configPath = cfg.Et.ConfigPath
		outputDir = cfg.Et.OutputDir
	}
	return et.NewCLIDelegator("", configPath, outputDir)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
