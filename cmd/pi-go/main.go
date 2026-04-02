// pi-go is a Go rewrite of the pi coding agent — an LLM-powered coding assistant.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/meganerd/pi-go/internal/agent"
	"github.com/meganerd/pi-go/internal/compact"
	"github.com/meganerd/pi-go/internal/config"
	"github.com/meganerd/pi-go/internal/message"
	projctx "github.com/meganerd/pi-go/internal/context"
	"github.com/meganerd/pi-go/internal/et"
	"github.com/meganerd/pi-go/internal/provider"
	"github.com/meganerd/pi-go/internal/provider/anthropic"
	"github.com/meganerd/pi-go/internal/render"
	"github.com/meganerd/pi-go/internal/provider/gemini"
	"github.com/meganerd/pi-go/internal/provider/openai"
	"github.com/meganerd/pi-go/internal/session"
	"github.com/meganerd/pi-go/internal/tool"
	"github.com/meganerd/pi-go/internal/tui"
	"github.com/meganerd/pi-go/internal/version"
)

func main() {
	var (
		showVersion  bool
		listSessions bool
		model        string
		providerArg  string
		sessionDir   string
		resume       bool
		prompt       string
		toolsArg     string
		noTools      bool
		sessionID    string
	)

	flag.BoolVar(&showVersion, "version", false, "Print version and exit")
	flag.BoolVar(&listSessions, "list-sessions", false, "List recent sessions and exit")
	flag.StringVar(&model, "model", "", "LLM model to use")
	flag.StringVar(&providerArg, "provider", "", "LLM provider (anthropic, openai, openrouter, gemini)")
	flag.StringVar(&sessionDir, "session-dir", "", "Session storage directory")
	flag.BoolVar(&resume, "resume", false, "Resume last session for current directory")
	flag.StringVar(&prompt, "prompt", "", "Send a single prompt and exit (use - for stdin)")
	flag.StringVar(&toolsArg, "tools", "", "Comma-separated list of enabled tools (e.g. read,grep,find,ls)")
	flag.BoolVar(&noTools, "no-tools", false, "Disable all tools")
	flag.StringVar(&sessionID, "session-id", "", "Resume a specific session by ID")
	flag.Parse()

	if showVersion {
		fmt.Println(version.Info())
		os.Exit(0)
	}

	if listSessions {
		listAllSessions(sessionDir)
		os.Exit(0)
	}

	// Load and merge configuration
	cfg := config.Load()
	cfg.ApplyFlags(model, providerArg, sessionDir)

	// Validate configuration and print warnings
	if warnings := cfg.Validate(); len(warnings) > 0 {
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
		}
	}

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

	// Apply tool filtering from CLI flags
	if noTools {
		tools = nil
	} else if toolsArg != "" {
		tools = filterTools(tools, strings.Split(toolsArg, ","))
	} else if len(cfg.Tools) > 0 {
		tools = filterTools(tools, cfg.Tools)
	}

	// Add et_delegate tool if electrictown is available
	etDelegator := initEtDelegator(cfg)
	if etDelegator != nil && etDelegator.Available() {
		tools = append(tools, &tool.EtDelegateTool{Delegator: etDelegator})
		fmt.Fprintln(os.Stderr, "et: electrictown integration active")
	}

	// Initialize session
	sess, err := initSession(cfg, resume, sessionID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: session unavailable: %v\n", err)
	}

	// Build compactor for context management
	compactor := compact.New(prov, cfg.Model, cfg.MaxContextTokens, 0)

	// Build agent loop with tool activity callback, confirmation, compaction, and streaming
	loop := agent.New(prov, tools).
		WithCompactor(compactor).
		WithConfirmCallback(func(name string, input json.RawMessage) bool {
			fmt.Fprintf(os.Stderr, "Allow %s? [Y/n] ", name)
			var answer string
			fmt.Scanln(&answer)
			answer = strings.TrimSpace(strings.ToLower(answer))
			return answer == "" || answer == "y" || answer == "yes"
		}).
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

	// Build system prompt with project context
	cwd, _ := os.Getwd()

	// SYSTEM.md replaces the default prompt; APPEND_SYSTEM.md appends to it.
	sysMd, appendMd := projctx.DiscoverSystemPromptFiles(cwd)

	systemPrompt := cfg.SystemPrompt
	if systemPrompt == "" {
		if sysMd != "" {
			systemPrompt = sysMd
			fmt.Fprintln(os.Stderr, "Loaded SYSTEM.md (replaces default prompt)")
		} else {
			systemPrompt = tui.DefaultSystemPrompt(tools)
		}
	}
	if appendMd != "" {
		systemPrompt += "\n\n" + appendMd
		fmt.Fprintln(os.Stderr, "Loaded APPEND_SYSTEM.md")
	}

	if pctx, err := projctx.Discover(cwd); err == nil {
		if extra := pctx.ForSystemPrompt(); extra != "" {
			systemPrompt += extra
			fmt.Fprintf(os.Stderr, "Loaded %d project context file(s)\n", len(pctx.Files))
		}
	}
	if gitCtx := projctx.GitContext(cwd); gitCtx != "" {
		systemPrompt += gitCtx
	}

	// Non-interactive mode: --prompt flag
	if prompt != "" {
		runPrompt(prompt, loop, cfg, systemPrompt, sess)
		return
	}

	// Run TUI
	opts := []tui.Option{
		tui.WithModel(cfg.Model),
		tui.WithSystemPrompt(systemPrompt),
		tui.WithMaxTokens(cfg.MaxTokens),
		tui.WithMaxContextTokens(cfg.MaxContextTokens),
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

func runPrompt(prompt string, loop *agent.Loop, cfg *config.Config, systemPrompt string, sess session.Store) {
	// Read from stdin if prompt is "-"
	if prompt == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
			os.Exit(1)
		}
		prompt = strings.TrimSpace(string(data))
		if prompt == "" {
			fmt.Fprintln(os.Stderr, "Error: empty input from stdin")
			os.Exit(1)
		}
	}

	req := &provider.ChatRequest{
		Model:        cfg.Model,
		SystemPrompt: systemPrompt,
		MaxTokens:    cfg.MaxTokens,
		Messages: []message.Message{
			{Role: message.RoleUser, Content: prompt},
		},
	}

	resp, err := loop.Run(context.Background(), req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		if sess != nil {
			sess.Close()
		}
		os.Exit(1)
	}

	// Print response — skip if streaming already printed tokens
	if !provider.CanStream(loop.Provider()) && resp.Message.Content != "" {
		fmt.Println(render.Markdown(resp.Message.Content))
	} else if provider.CanStream(loop.Provider()) {
		fmt.Println() // trailing newline after streamed content
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
	reg.Register("zai", func(apiKey, _ string) provider.Provider {
		return openai.NewZAI(apiKey)
	})
	reg.Register("gemini", func(apiKey, baseURL string) provider.Provider {
		return gemini.New(apiKey, baseURL)
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
	case "zai":
		apiKey = os.Getenv("ZAI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("ZAI_API_KEY environment variable not set. Set it with: export ZAI_API_KEY=your-key")
		}
	case "gemini":
		apiKey = os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY environment variable not set. Set it with: export GEMINI_API_KEY=your-key")
		}
	default:
		return nil, fmt.Errorf("unknown provider %q. Available: anthropic, openai, openrouter, gemini, zai", cfg.Provider)
	}

	prov := reg.Get(cfg.Provider, apiKey, "")
	if prov == nil {
		return nil, fmt.Errorf("failed to initialize provider %q", cfg.Provider)
	}
	return prov, nil
}

func initSession(cfg *config.Config, resume bool, sessionID string) (session.Store, error) {
	if err := os.MkdirAll(cfg.SessionDir, 0755); err != nil {
		return nil, fmt.Errorf("create session directory: %w", err)
	}

	mgr := session.NewManager(cfg.SessionDir)

	// --session-id takes precedence over --resume
	if sessionID != "" {
		store, err := mgr.Open(sessionID)
		if err != nil {
			return nil, fmt.Errorf("open session %q: %w", sessionID, err)
		}
		fmt.Fprintf(os.Stderr, "Resumed session: %s\n", sessionID)
		return store, nil
	}

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

func listAllSessions(sessionDir string) {
	if sessionDir == "" {
		home, _ := os.UserHomeDir()
		sessionDir = filepath.Join(home, ".config", "pi-go", "sessions")
	}
	mgr := session.NewManager(sessionDir)
	sessions, err := mgr.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	if len(sessions) == 0 {
		fmt.Println("No sessions found")
		return
	}
	fmt.Printf("%-20s %-40s %s\n", "ID", "Path", "Created")
	fmt.Printf("%-20s %-40s %s\n", strings.Repeat("-", 20), strings.Repeat("-", 40), strings.Repeat("-", 20))
	for _, s := range sessions {
		path := s.Path
		if len(path) > 40 {
			path = "..." + path[len(path)-37:]
		}
		created := s.CreatedAt
		if len(created) > 19 {
			created = created[:19]
		}
		fmt.Printf("%-20s %-40s %s\n", s.ID, path, created)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// filterTools returns only tools whose names appear in the allowed list.
func filterTools(tools []tool.Tool, allowed []string) []tool.Tool {
	set := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		set[strings.TrimSpace(name)] = true
	}
	var filtered []tool.Tool
	for _, t := range tools {
		if set[t.Name()] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}
