package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/meganerd/pi-go/internal/agent"
	"github.com/meganerd/pi-go/internal/message"
	"github.com/meganerd/pi-go/internal/provider"
	"github.com/meganerd/pi-go/internal/render"
	"github.com/meganerd/pi-go/internal/session"
	"github.com/meganerd/pi-go/internal/usage"
)

// --- Messages (bubbletea events) ---

type agentResponseMsg struct {
	resp *provider.ChatResponse
	err  error
}

type streamTokenMsg string
type toolCallMsg struct {
	name     string
	isResult bool
	output   string
	isError  bool
}

type confirmToolMsg struct {
	name  string
	input string
	reply chan bool
}

// --- Styles ---

var (
	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)
	userStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	assistantStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	toolStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Italic(true)
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

// Model is the bubbletea model for the pi-go TUI.
type Model struct {
	agent       *agent.Loop
	sess        session.Store
	tracker     *usage.Tracker
	model       string
	systemPr    string
	maxTokens   int
	maxCtxToks  int
	streaming   bool
	isStreaming  bool // true when currently streaming a response

	textarea  textarea.Model
	viewport  viewport.Model
	content   strings.Builder // accumulated conversation display
	width     int
	height    int
	ready     bool
	quitting  bool
	waiting   bool // true when waiting for agent response
	cleared   bool

	// Confirmation state
	confirming bool
	confirmMsg confirmToolMsg
}

// NewModel creates a new bubbletea Model for pi-go.
func NewModel(agentLoop *agent.Loop, opts ...Option) Model {
	// Use the old Option type to gather config
	t := &TUI{}
	for _, opt := range opts {
		opt(t)
	}

	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.Focus()
	ta.CharLimit = 0 // no limit
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	tracker := usage.New(t.model)
	if t.maxContextTokens > 0 {
		tracker.SetBudget(t.maxContextTokens)
	}

	return Model{
		agent:      agentLoop,
		sess:       t.session,
		tracker:    tracker,
		model:      t.model,
		systemPr:   t.system,
		maxTokens:  t.maxTokens,
		maxCtxToks: t.maxContextTokens,
		streaming:  t.streaming,
		textarea:   ta,
	}
}

func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.confirming {
			return m.handleConfirmKey(msg)
		}
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyCtrlD:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyCtrlG:
			// Open external editor
			return m, tea.ExecProcess(editorCmd(), func(err error) tea.Msg {
				if err != nil {
					return agentResponseMsg{err: fmt.Errorf("editor: %w", err)}
				}
				return nil
			})
		case tea.KeyEnter:
			if m.waiting {
				return m, nil // ignore input while waiting
			}
			return m.handleSubmit()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textarea.SetWidth(msg.Width)
		footerHeight := 1
		inputHeight := 5
		vpHeight := msg.Height - footerHeight - inputHeight
		if vpHeight < 3 {
			vpHeight = 3
		}
		if !m.ready {
			m.viewport = viewport.New(msg.Width, vpHeight)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = vpHeight
		}
		return m, nil

	case agentResponseMsg:
		m.waiting = false
		m.isStreaming = false
		if msg.err != nil {
			m.appendContent(errorStyle.Render(fmt.Sprintf("Error: %v", msg.err)))
		} else {
			if !m.streaming && msg.resp.Message.Content != "" {
				m.appendContent(assistantStyle.Render("Assistant") + "\n" + render.Markdown(msg.resp.Message.Content))
			}
			if msg.resp.Usage.InputTokens > 0 || msg.resp.Usage.OutputTokens > 0 {
				m.tracker.Add(msg.resp.Usage.InputTokens, msg.resp.Usage.OutputTokens)
			}
		}
		m.syncViewport()
		m.textarea.Focus()
		return m, nil

	case streamTokenMsg:
		m.isStreaming = true
		m.content.WriteString(string(msg))
		m.syncViewport()
		return m, nil

	case toolCallMsg:
		if !msg.isResult {
			m.appendContent(toolStyle.Render(fmt.Sprintf("[tool: %s]", msg.name)))
		} else if msg.isError {
			m.appendContent(toolStyle.Render(fmt.Sprintf("[%s error: %s]", msg.name, truncateStr(msg.output, 200))))
		} else {
			m.appendContent(toolStyle.Render(fmt.Sprintf("[%s done: %d bytes]", msg.name, len(msg.output))))
		}
		m.syncViewport()
		return m, nil

	case confirmToolMsg:
		m.confirming = true
		m.confirmMsg = msg
		return m, nil
	}

	// Update textarea
	var taCmd tea.Cmd
	m.textarea, taCmd = m.textarea.Update(msg)
	cmds = append(cmds, taCmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.quitting {
		stats := m.tracker.Stats()
		if stats.Calls > 0 {
			return fmt.Sprintf("Usage: %s\nGoodbye!\n", stats)
		}
		return "Goodbye!\n"
	}

	if !m.ready {
		return "Initializing..."
	}

	// Viewport (conversation)
	vpView := m.viewport.View()

	// Confirmation prompt
	if m.confirming {
		vpView += fmt.Sprintf("\n  Allow %s? [Y/n] ", m.confirmMsg.name)
	}

	// Input area
	inputView := m.textarea.View()

	// Footer
	footer := m.renderFooter()

	return fmt.Sprintf("%s\n%s\n%s", vpView, inputView, footer)
}

func (m *Model) renderFooter() string {
	stats := m.tracker.Stats()
	var parts []string
	parts = append(parts, m.model)
	parts = append(parts, fmt.Sprintf("%d tok", stats.TotalTokens))
	if stats.HasPricing {
		parts = append(parts, fmt.Sprintf("$%.4f", stats.EstCost))
	}
	if stats.Budget > 0 {
		pct := float64(stats.TotalTokens) / float64(stats.Budget) * 100
		parts = append(parts, fmt.Sprintf("%.0f%% ctx", pct))
	}
	cwd, _ := os.Getwd()
	if len(cwd) > 30 {
		cwd = "..." + cwd[len(cwd)-27:]
	}
	parts = append(parts, cwd)

	line := strings.Join(parts, " | ")

	// Pad to terminal width
	if m.width > 0 && len(line) < m.width {
		line = line + strings.Repeat(" ", m.width-len(line))
	}

	return footerStyle.Render(line)
}

func (m *Model) handleSubmit() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.textarea.Value())
	m.textarea.Reset()

	if input == "" {
		return m, nil
	}

	// Shell commands
	if strings.HasPrefix(input, "!!") {
		output := execShellCmd(input[2:])
		if output != "" {
			m.appendContent(output)
			m.syncViewport()
		}
		return m, nil
	}
	if strings.HasPrefix(input, "!") {
		cmdStr := input[1:]
		output := execShellCmd(cmdStr)
		if output != "" {
			m.appendContent(output)
		}
		if output != "" {
			input = fmt.Sprintf("I ran `%s` and got this output:\n\n```\n%s\n```", cmdStr, output)
		} else {
			return m, nil
		}
	}

	// Commands
	if strings.HasPrefix(input, "/") {
		return m.handleCommand(input)
	}

	// Show user message in viewport
	m.appendContent(userStyle.Render("You") + "\n" + input)
	if m.streaming {
		m.appendContent(assistantStyle.Render("Assistant"))
	}
	m.syncViewport()

	// Send to agent
	m.waiting = true
	m.textarea.Blur()

	return m, m.sendMessage(input)
}

func (m *Model) handleCommand(input string) (tea.Model, tea.Cmd) {
	switch {
	case input == "/exit":
		m.quitting = true
		return m, tea.Quit
	case input == "/help":
		m.appendContent(m.helpText())
	case input == "/usage":
		stats := m.tracker.Stats()
		if stats.Calls == 0 {
			m.appendContent("No usage yet")
		} else {
			m.appendContent(fmt.Sprintf("Usage: %s", stats))
		}
	case input == "/model":
		m.appendContent(fmt.Sprintf("Model: %s", m.model))
	case input == "/clear":
		m.content.Reset()
		m.cleared = true
		m.appendContent("Conversation cleared.")
	case input == "/session":
		if m.sess == nil {
			m.appendContent("No active session")
		} else {
			msgs, err := m.sess.Messages()
			if err != nil {
				m.appendContent(fmt.Sprintf("Session error: %v", err))
			} else {
				m.appendContent(fmt.Sprintf("Session: %d messages", len(msgs)))
			}
		}
	case input == "/tree":
		if m.sess == nil {
			m.appendContent("No active session")
		} else {
			msgs, _ := m.sess.Messages()
			m.appendContent(session.TreeView(msgs))
		}
	case input == "/fork":
		if m.sess == nil {
			m.appendContent("No active session to fork")
		} else {
			msgs, _ := m.sess.Messages()
			if len(msgs) > 0 {
				lastID := msgs[len(msgs)-1].ID
				if err := m.sess.Branch(lastID); err != nil {
					m.appendContent(fmt.Sprintf("Fork error: %v", err))
				} else {
					m.appendContent(fmt.Sprintf("Forked from message %s", lastID))
				}
			}
		}
	case strings.HasPrefix(input, "/export"):
		path := strings.TrimSpace(strings.TrimPrefix(input, "/export"))
		if path == "" {
			path = "session-export.md"
		}
		if m.sess == nil {
			m.appendContent("No active session to export")
		} else {
			msgs, _ := m.sess.Messages()
			md := session.ExportMarkdown(msgs)
			if err := os.WriteFile(path, []byte(md), 0600); err != nil {
				m.appendContent(fmt.Sprintf("Export error: %v", err))
			} else {
				m.appendContent(fmt.Sprintf("Exported to %s (%d messages)", path, len(msgs)))
			}
		}
	case strings.HasPrefix(input, "/name "):
		name := strings.TrimSpace(input[6:])
		m.appendContent(fmt.Sprintf("Session named: %s", name))
	case input == "/share":
		if m.sess == nil {
			m.appendContent("No active session to share")
		} else {
			msgs, _ := m.sess.Messages()
			md := session.ExportMarkdown(msgs)
			result, err := Share("pi-go session", md)
			if err != nil {
				m.appendContent(fmt.Sprintf("Share error: %v", err))
			} else {
				m.appendContent(fmt.Sprintf("Shared to %s: %s", result.Platform, result.URL))
			}
		}
	case input == "/compact":
		compactor := m.agent.Compactor()
		if compactor == nil {
			m.appendContent("Compaction not available")
		} else if m.sess == nil {
			m.appendContent("No active session")
		} else {
			msgs, _ := m.sess.Messages()
			tokens := compactor.EstimateTokens(msgs)
			m.appendContent(fmt.Sprintf("Context: ~%d tokens across %d messages", tokens, len(msgs)))
		}
	default:
		m.appendContent(fmt.Sprintf("Unknown command: %s", input))
	}
	m.syncViewport()
	return m, nil
}

func (m *Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		m.confirming = false
		m.confirmMsg.reply <- true
		return m, nil
	case "n", "N", "esc":
		m.confirming = false
		m.confirmMsg.reply <- false
		return m, nil
	}
	return m, nil
}

func (m *Model) sendMessage(input string) tea.Cmd {
	return func() tea.Msg {
		userMsg := message.Message{
			Role:    message.RoleUser,
			Content: input,
		}

		req := &provider.ChatRequest{
			Model:        m.model,
			SystemPrompt: m.systemPr,
			MaxTokens:    m.maxTokens,
		}
		if m.sess != nil && !m.cleared {
			if err := m.agent.Resume(req); err != nil {
				// Non-fatal
			}
			_ = m.sess.Append(&userMsg)
		}
		m.cleared = false
		req.Messages = append(req.Messages, userMsg)

		resp, err := m.agent.Run(context.Background(), req)
		return agentResponseMsg{resp: resp, err: err}
	}
}

func (m *Model) appendContent(s string) {
	if m.content.Len() > 0 {
		m.content.WriteString("\n")
	}
	m.content.WriteString(s)
}

func (m *Model) syncViewport() {
	m.viewport.SetContent(m.content.String())
	m.viewport.GotoBottom()
}

func (m *Model) helpText() string {
	return `Available commands:
  /help        Show this help
  /exit        Exit pi-go
  /session     Show session info
  /usage       Show token usage and cost
  /model       Show current model
  /clear       Clear conversation history
  /compact     Show context and compaction status
  /tree        Show session message tree
  /fork        Create branch from current position
  /export [f]  Export session to markdown file
  /share       Share session (GitLab snippet or GitHub gist)
  /name <n>    Set session display name

Shell commands:
  !command     Run and send output to LLM
  !!command    Run without sending to LLM

Keyboard shortcuts:
  Ctrl+G       Open $EDITOR for long input
  Ctrl+C/D     Exit`
}

func execShellCmd(cmdStr string) string {
	cmdStr = strings.TrimSpace(cmdStr)
	if cmdStr == "" {
		return ""
	}
	cmd := exec.Command("bash", "-c", cmdStr) //nolint:gosec // User-initiated shell command
	out, _ := cmd.CombinedOutput()
	return strings.TrimRight(string(out), "\n")
}

// editorCmd returns an exec.Cmd for the user's preferred editor.
func editorCmd() *exec.Cmd {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	tmpFile, err := os.CreateTemp("", "pi-go-edit-*.md")
	if err != nil {
		return exec.Command("echo", "failed to create temp file")
	}
	_ = tmpFile.Close()
	return exec.Command(editor, tmpFile.Name()) //nolint:gosec // User-configured editor
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// RunBubbleTea starts the bubbletea TUI program.
func RunBubbleTea(agentLoop *agent.Loop, opts ...Option) error {
	m := NewModel(agentLoop, opts...)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
