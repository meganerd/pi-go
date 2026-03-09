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
	"github.com/meganerd/pi-go/internal/session"
)

// TUI manages the interactive conversation loop.
type TUI struct {
	agent   *agent.Loop
	session session.Store
	model   string
	system  string

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
			fmt.Fprintln(t.out, "\nGoodbye!")
			return nil
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
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
	fmt.Fprintln(t.out, "Type /exit to quit, Ctrl+D for EOF")
}

func (t *TUI) handleCommand(input string) (handled bool, err error) {
	switch {
	case input == "/exit":
		fmt.Fprintln(t.out, "Goodbye!")
		return true, io.EOF
	case input == "/session":
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

func (t *TUI) handleMessage(ctx context.Context, input string) error {
	userMsg := message.Message{
		Role:    message.RoleUser,
		Content: input,
	}

	// Build request: load session history + append new user message
	req := &provider.ChatRequest{
		Model:        t.model,
		SystemPrompt: t.system,
	}
	if t.session != nil {
		if err := t.agent.Resume(req); err != nil {
			fmt.Fprintf(t.err, "Warning: could not resume session: %v\n", err)
		}
		_ = t.session.Append(&userMsg)
	}
	req.Messages = append(req.Messages, userMsg)

	// Show spinner while waiting
	stop := t.startSpinner(ctx)

	resp, err := t.agent.Run(ctx, req)
	stop()

	if err != nil {
		return err
	}

	// Print assistant response
	if resp.Message.Content != "" {
		fmt.Fprintln(t.out)
		fmt.Fprintln(t.out, resp.Message.Content)
	}

	// Print usage
	if resp.Usage.InputTokens > 0 || resp.Usage.OutputTokens > 0 {
		fmt.Fprintf(t.err, "[%d in / %d out tokens]\n", resp.Usage.InputTokens, resp.Usage.OutputTokens)
	}

	return nil
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
