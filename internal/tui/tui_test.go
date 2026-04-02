package tui

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/meganerd/pi-go/internal/agent"
	"github.com/meganerd/pi-go/internal/message"
	"github.com/meganerd/pi-go/internal/provider"
)

// mockProvider implements provider.Provider for testing.
type mockProvider struct {
	response *provider.ChatResponse
	err      error
}

func (m *mockProvider) Name() string          { return "mock" }
func (m *mockProvider) Models() []provider.Model { return nil }
func (m *mockProvider) Chat(_ context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.response != nil {
		return m.response, nil
	}
	// Default: echo back the last user message
	lastMsg := ""
	for _, msg := range req.Messages {
		if msg.Role == message.RoleUser {
			lastMsg = msg.Content
		}
	}
	return &provider.ChatResponse{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: "Echo: " + lastMsg,
		},
		Usage: provider.Usage{InputTokens: 10, OutputTokens: 5},
	}, nil
}

func TestTUI_ExitCommand(t *testing.T) {
	in := strings.NewReader("/exit\n")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	loop := agent.New(&mockProvider{}, nil)
	ui := New(loop, WithIO(in, out, errOut), WithModel("test-model"))

	err := ui.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Goodbye!") {
		t.Errorf("output should contain Goodbye!, got: %s", out.String())
	}
}

func TestTUI_EOF(t *testing.T) {
	in := strings.NewReader("") // immediate EOF
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	loop := agent.New(&mockProvider{}, nil)
	ui := New(loop, WithIO(in, out, errOut))

	err := ui.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Goodbye!") {
		t.Errorf("output should contain Goodbye!, got: %s", out.String())
	}
}

func TestTUI_EmptyInput(t *testing.T) {
	// Empty line followed by /exit
	in := strings.NewReader("\n\n/exit\n")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	loop := agent.New(&mockProvider{}, nil)
	ui := New(loop, WithIO(in, out, errOut))

	err := ui.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not have sent anything to provider (no "Echo:" in output)
	if strings.Contains(out.String(), "Echo:") {
		t.Error("empty input should not trigger LLM call")
	}
}

func TestTUI_MessageExchange(t *testing.T) {
	in := strings.NewReader("hello world\n/exit\n")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	loop := agent.New(&mockProvider{}, nil)
	ui := New(loop, WithIO(in, out, errOut), WithModel("test-model"))

	err := ui.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Echo: hello world") {
		t.Errorf("should echo user input, got: %s", out.String())
	}
}

func TestTUI_SessionCommand(t *testing.T) {
	in := strings.NewReader("/session\n/exit\n")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	loop := agent.New(&mockProvider{}, nil)
	ui := New(loop, WithIO(in, out, errOut))

	err := ui.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "No active session") {
		t.Errorf("should show no active session, got: %s", out.String())
	}
}

func TestTUI_UnknownCommand(t *testing.T) {
	in := strings.NewReader("/foobar\n/exit\n")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	loop := agent.New(&mockProvider{}, nil)
	ui := New(loop, WithIO(in, out, errOut))

	err := ui.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Unknown command: /foobar") {
		t.Errorf("should show unknown command, got: %s", out.String())
	}
}

func TestTUI_WelcomeShowsModel(t *testing.T) {
	in := strings.NewReader("/exit\n")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	loop := agent.New(&mockProvider{}, nil)
	ui := New(loop, WithIO(in, out, errOut), WithModel("claude-test"))

	_ = ui.Run(context.Background())

	if !strings.Contains(out.String(), "claude-test") {
		t.Errorf("welcome should show model name, got: %s", out.String())
	}
}

func TestTUI_ProviderError(t *testing.T) {
	in := strings.NewReader("test\n/exit\n")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	loop := agent.New(&mockProvider{err: io.ErrUnexpectedEOF}, nil)
	ui := New(loop, WithIO(in, out, errOut))

	err := ui.Run(context.Background())
	if err != nil {
		t.Fatalf("should not return error for provider failure: %v", err)
	}

	if !strings.Contains(errOut.String(), "Error:") {
		t.Errorf("should show error on stderr, got: %s", errOut.String())
	}
}

func TestTUI_StatusLine(t *testing.T) {
	in := strings.NewReader("test\n/exit\n")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	loop := agent.New(&mockProvider{}, nil)
	ui := New(loop, WithIO(in, out, errOut), WithModel("test-model"))

	_ = ui.Run(context.Background())

	// Should show status line with model and tokens
	if !strings.Contains(errOut.String(), "test-model") {
		t.Errorf("status line should contain model name, got: %s", errOut.String())
	}
	if !strings.Contains(errOut.String(), "15 tok") {
		t.Errorf("status line should contain token count, got: %s", errOut.String())
	}
}

func TestFormatStatusLine(t *testing.T) {
	line := FormatStatusLine("opus", 150, 0.0234, true, 42.5, true)
	if !strings.Contains(line, "opus") {
		t.Errorf("should contain model: %s", line)
	}
	if !strings.Contains(line, "150 tok") {
		t.Errorf("should contain tokens: %s", line)
	}
	if !strings.Contains(line, "$0.0234") {
		t.Errorf("should contain cost: %s", line)
	}
	if !strings.Contains(line, "42% ctx") {
		t.Errorf("should contain context pct: %s", line)
	}
}

func TestFormatStatusLine_NoPricing(t *testing.T) {
	line := FormatStatusLine("unknown-model", 100, 0, false, 0, false)
	if strings.Contains(line, "$") {
		t.Errorf("should not contain cost when no pricing: %s", line)
	}
	if strings.Contains(line, "ctx") {
		t.Errorf("should not contain ctx when no budget: %s", line)
	}
}

func TestTUI_HelpCommand(t *testing.T) {
	in := strings.NewReader("/help\n/exit\n")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	loop := agent.New(&mockProvider{}, nil)
	ui := New(loop, WithIO(in, out, errOut))

	_ = ui.Run(context.Background())

	if !strings.Contains(out.String(), "Available commands") {
		t.Errorf("should show help, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "/usage") {
		t.Errorf("help should list /usage command, got: %s", out.String())
	}
}

func TestTUI_ModelCommand(t *testing.T) {
	in := strings.NewReader("/model\n/exit\n")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	loop := agent.New(&mockProvider{}, nil)
	ui := New(loop, WithIO(in, out, errOut), WithModel("test-model-x"))

	_ = ui.Run(context.Background())

	if !strings.Contains(out.String(), "Model: test-model-x") {
		t.Errorf("should show model, got: %s", out.String())
	}
}

func TestTUI_ClearCommand(t *testing.T) {
	in := strings.NewReader("/clear\n/exit\n")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	loop := agent.New(&mockProvider{}, nil)
	ui := New(loop, WithIO(in, out, errOut))

	_ = ui.Run(context.Background())

	if !strings.Contains(out.String(), "Conversation cleared") {
		t.Errorf("should confirm clear, got: %s", out.String())
	}
}

func TestTUI_MultilineInput(t *testing.T) {
	in := strings.NewReader("```\nline one\nline two\n```\n/exit\n")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	loop := agent.New(&mockProvider{}, nil)
	ui := New(loop, WithIO(in, out, errOut))

	_ = ui.Run(context.Background())

	if !strings.Contains(out.String(), "Echo: line one\nline two") {
		t.Errorf("should echo multiline input, got: %s", out.String())
	}
}

func TestTUI_ShellCommand_DoubleExclaim(t *testing.T) {
	in := strings.NewReader("!!echo hello shell\n/exit\n")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	loop := agent.New(&mockProvider{}, nil)
	ui := New(loop, WithIO(in, out, errOut))

	_ = ui.Run(context.Background())

	if !strings.Contains(out.String(), "hello shell") {
		t.Errorf("!! should show command output, got: %s", out.String())
	}
	// Should NOT send to LLM (no "Echo:" in output)
	if strings.Contains(out.String(), "Echo:") {
		t.Error("!! should not send to LLM")
	}
}

func TestTUI_ShellCommand_SingleExclaim(t *testing.T) {
	in := strings.NewReader("!echo test output\n/exit\n")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	loop := agent.New(&mockProvider{}, nil)
	ui := New(loop, WithIO(in, out, errOut))

	_ = ui.Run(context.Background())

	// Should show command output
	if !strings.Contains(out.String(), "test output") {
		t.Errorf("! should show command output, got: %s", out.String())
	}
	// Should also send to LLM (echo back)
	if !strings.Contains(out.String(), "Echo:") {
		t.Errorf("! should send output to LLM, got: %s", out.String())
	}
}

func TestTUI_NameCommand(t *testing.T) {
	in := strings.NewReader("/name my-session\n/exit\n")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	loop := agent.New(&mockProvider{}, nil)
	ui := New(loop, WithIO(in, out, errOut))

	_ = ui.Run(context.Background())

	if !strings.Contains(out.String(), "Session named: my-session") {
		t.Errorf("should confirm session name, got: %s", out.String())
	}
}

func TestTUI_HelpShowsShellCommands(t *testing.T) {
	in := strings.NewReader("/help\n/exit\n")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	loop := agent.New(&mockProvider{}, nil)
	ui := New(loop, WithIO(in, out, errOut))

	_ = ui.Run(context.Background())

	if !strings.Contains(out.String(), "!command") {
		t.Errorf("help should show shell commands, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "/name") {
		t.Errorf("help should show /name command, got: %s", out.String())
	}
}

func TestTUI_UsageCommand(t *testing.T) {
	in := strings.NewReader("test\n/usage\n/exit\n")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	loop := agent.New(&mockProvider{}, nil)
	ui := New(loop, WithIO(in, out, errOut))

	_ = ui.Run(context.Background())

	if !strings.Contains(out.String(), "15 tokens") {
		t.Errorf("should show cumulative usage, got: %s", out.String())
	}
}
