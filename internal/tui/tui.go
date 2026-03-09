// Package tui implements the interactive terminal conversation loop for pi-go.
package tui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/meganerd/pi-go/internal/agent"
	"github.com/meganerd/pi-go/internal/message"
	"github.com/meganerd/pi-go/internal/provider"
	"github.com/meganerd/pi-go/internal/render"
	"github.com/meganerd/pi-go/internal/session"
	"github.com/meganerd/pi-go/internal/usage"
)

// TUI manages the interactive conversation loop.
type TUI struct {
	agent     *agent.Loop
	session   session.Store
	tracker   *usage.Tracker
	model     string
	system    string
	maxTokens int  // max output tokens per LLM call
	streaming bool // true when agent loop has a stream callback
	cleared   bool // set by /clear to skip session history on next message

	in  io.Reader
	out io.Writer
	err io.Writer
}

// New creates a TUI with the given agent loop and configuration.
func New(agentLoop *agent.Loop, opts ...Option) *TUI {
	t := &TUI{
		agent: agentLoop,
		in:    os.Stdin,
		out:   os.Stdout,
		err:   os.Stderr,
	}
	for _, opt := range opts {
		opt(t)
	}
	t.tracker = usage.New(t.model)
	return t
}

// Option configures a TUI instance.
type Option func(*TUI)

// WithModel sets the model name for display and requests.
func WithModel(model string) Option {
	return func(t *TUI) { t.model = model }
}

// WithSystemPrompt sets the system prompt.
func WithSystemPrompt(prompt string) Option {
	return func(t *TUI) { t.system = prompt }
}

// WithSession sets the session store.
func WithSession(s session.Store) Option {
	return func(t *TUI) { t.session = s }
}

// WithMaxTokens sets the maximum output tokens per LLM call.
func WithMaxTokens(n int) Option {
	return func(t *TUI) { t.maxTokens = n }
}

// WithStreaming marks the TUI as having streaming active (agent loop handles output).
func WithStreaming() Option {
	return func(t *TUI) { t.streaming = true }
}

// WithIO overrides stdin/stdout/stderr for testing.
func WithIO(in io.Reader, out, errOut io.Writer) Option {
	return func(t *TUI) {
		t.in = in
		t.out = out
		t.err = errOut
	}
}

// Run starts the interactive conversation loop. It blocks until the user
// exits or the context is cancelled.
func (t *TUI) Run(ctx context.Context) error {
	// Set up Ctrl+C handling
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	t.printWelcome()

	scanner := bufio.NewScanner(t.in)
	for {
		fmt.Fprint(t.out, "\n> ")

		if !scanner.Scan() {
			// EOF (Ctrl+D)
			t.printUsageStats()
			fmt.Fprintln(t.out, "\nGoodbye!")
			return nil
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Multiline input: collect lines between ``` delimiters
		if input == "```" {
			input = t.readMultiline(scanner)
			if input == "" {
				continue
			}
		}

		// Handle commands
		if handled, err := t.handleCommand(input); handled {
			if err == io.EOF {
				return nil // clean exit via /exit
			}
			if err != nil {
				return err
			}
			continue
		}

		// Send to agent
		if err := t.handleMessage(ctx, input); err != nil {
			if ctx.Err() != nil {
				fmt.Fprintln(t.out, "\nInterrupted. Goodbye!")
				return nil
			}
			fmt.Fprintf(t.err, "Error: %v\n", err)
		}
	}
}

func (t *TUI) printWelcome() {
	fmt.Fprintln(t.out, "pi-go — LLM-powered coding assistant")
	if t.model != "" {
		fmt.Fprintf(t.out, "Model: %s\n", t.model)
	}
	fmt.Fprintln(t.out, "Type /help for commands")
}

func (t *TUI) printUsageStats() {
	stats := t.tracker.Stats()
	if stats.Calls == 0 {
		fmt.Fprintln(t.out, "No usage yet")
		return
	}
	fmt.Fprintf(t.out, "Usage: %s\n", stats)
}

func (t *TUI) handleCommand(input string) (handled bool, err error) {
	switch {
	case input == "/exit":
		t.printUsageStats()
		fmt.Fprintln(t.out, "Goodbye!")
		return true, io.EOF
	case input == "/session":
		t.printSessionInfo()
		return true, nil
	case input == "/usage":
		t.printUsageStats()
		return true, nil
	case input == "/help":
		t.printHelp()
		return true, nil
	case input == "/clear":
		t.clearHistory()
		return true, nil
	case input == "/model":
		fmt.Fprintf(t.out, "Model: %s\n", t.model)
		return true, nil
	case input == "/compact":
		t.handleCompact()
		return true, nil
	case input == "/sessions":
		t.printSessionInfo()
		return true, nil
	default:
		if strings.HasPrefix(input, "/") {
			fmt.Fprintf(t.out, "Unknown command: %s\n", input)
			return true, nil
		}
		return false, nil
	}
}

func (t *TUI) printSessionInfo() {
	if t.session == nil {
		fmt.Fprintln(t.out, "No active session")
		return
	}
	msgs, err := t.session.Messages()
	if err != nil {
		fmt.Fprintf(t.err, "Session error: %v\n", err)
		return
	}
	fmt.Fprintf(t.out, "Session: %d messages\n", len(msgs))
}

func (t *TUI) handleCompact() {
	compactor := t.agent.Compactor()
	if compactor == nil {
		fmt.Fprintln(t.out, "Compaction not available (no compactor configured)")
		return
	}
	if t.session == nil {
		fmt.Fprintln(t.out, "No active session to compact")
		return
	}
	msgs, err := t.session.Messages()
	if err != nil {
		fmt.Fprintf(t.err, "Error reading session: %v\n", err)
		return
	}
	tokens := compactor.EstimateTokens(msgs)
	fmt.Fprintf(t.out, "Context: ~%d tokens across %d messages\n", tokens, len(msgs))
	if len(msgs) <= 10 {
		fmt.Fprintln(t.out, "Too few messages to compact")
		return
	}
	fmt.Fprintln(t.out, "Compaction will occur automatically when context exceeds the threshold.")
	fmt.Fprintf(t.out, "Current estimate: ~%d tokens\n", tokens)
}

func (t *TUI) printHelp() {
	fmt.Fprintln(t.out, "Available commands:")
	fmt.Fprintln(t.out, "  /help     Show this help")
	fmt.Fprintln(t.out, "  /exit     Exit pi-go")
	fmt.Fprintln(t.out, "  /session  Show session info")
	fmt.Fprintln(t.out, "  /usage    Show token usage and cost")
	fmt.Fprintln(t.out, "  /model    Show current model")
	fmt.Fprintln(t.out, "  /clear    Clear conversation history")
	fmt.Fprintln(t.out, "  /compact  Show context size and compaction status")
	fmt.Fprintln(t.out)
	fmt.Fprintln(t.out, "Paste multiline input using ``` delimiters:")
	fmt.Fprintln(t.out, "  > ```")
	fmt.Fprintln(t.out, "  your multiline")
	fmt.Fprintln(t.out, "  content here")
	fmt.Fprintln(t.out, "  ```")
}

func (t *TUI) clearHistory() {
	// Note: this doesn't clear the session file, just tells the user
	// the next message will start fresh in the conversation
	fmt.Fprintln(t.out, "Conversation cleared. Next message starts a fresh context.")
	// We signal this by setting a flag; handleMessage checks it
	t.cleared = true
}

func (t *TUI) handleMessage(ctx context.Context, input string) error {
	userMsg := message.Message{
		Role:    message.RoleUser,
		Content: input,
	}

	// Build request: load session history + append new user message
	req := &provider.ChatRequest{
		Model:        t.model,
		SystemPrompt: t.system,
		MaxTokens:    t.maxTokens,
	}
	if t.session != nil && !t.cleared {
		if err := t.agent.Resume(req); err != nil {
			fmt.Fprintf(t.err, "Warning: could not resume session: %v\n", err)
		}
		if err := t.session.Append(&userMsg); err != nil {
			fmt.Fprintf(t.err, "Warning: failed to persist message: %v\n", err)
		}
	}
	t.cleared = false
	req.Messages = append(req.Messages, userMsg)

	// Show spinner while waiting (skip if streaming — tokens will flow directly)
	var stop func()
	if !t.streaming {
		stop = t.startSpinner(ctx)
	}

	resp, err := t.agent.Run(ctx, req)
	if stop != nil {
		stop()
	}

	if err != nil {
		return err
	}

	// Print assistant response (skip if streaming — agent loop already printed tokens)
	if !t.streaming && resp.Message.Content != "" {
		fmt.Fprintln(t.out)
		fmt.Fprintln(t.out, render.Markdown(resp.Message.Content))
	} else if t.streaming {
		fmt.Fprintln(t.out) // newline after streamed content
	}

	// Track and print usage
	if resp.Usage.InputTokens > 0 || resp.Usage.OutputTokens > 0 {
		t.tracker.Add(resp.Usage.InputTokens, resp.Usage.OutputTokens)
		fmt.Fprintf(t.err, "[%d in / %d out tokens]\n", resp.Usage.InputTokens, resp.Usage.OutputTokens)
	}

	return nil
}

func (t *TUI) readMultiline(scanner *bufio.Scanner) string {
	fmt.Fprint(t.out, "... ")
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "```" {
			break
		}
		lines = append(lines, line)
		fmt.Fprint(t.out, "... ")
	}
	return strings.Join(lines, "\n")
}

func (t *TUI) startSpinner(ctx context.Context) func() {
	var once sync.Once
	done := make(chan struct{})
	exited := make(chan struct{})

	go func() {
		defer close(exited)
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		start := time.Now()
		i := 0
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				fmt.Fprintf(t.err, "\r\033[K") // clear spinner line
				return
			case <-ctx.Done():
				fmt.Fprintf(t.err, "\r\033[K")
				return
			case <-ticker.C:
				elapsed := time.Since(start).Seconds()
				fmt.Fprintf(t.err, "\r%s thinking... %.0fs", frames[i%len(frames)], elapsed)
				i++
			}
		}
	}()

	return func() {
		once.Do(func() { close(done) })
		<-exited // wait for goroutine to finish writing
	}
}
