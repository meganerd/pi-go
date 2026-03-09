package render

import (
	"strings"
	"testing"
)

func TestMarkdown_Headers(t *testing.T) {
	tests := []struct {
		input    string
		contains string
	}{
		{"# Title", "Title"},
		{"## Section", "Section"},
		{"### Subsection", "Subsection"},
	}
	for _, tt := range tests {
		result := Markdown(tt.input)
		if !strings.Contains(result, tt.contains) {
			t.Errorf("Markdown(%q) should contain %q, got %q", tt.input, tt.contains, result)
		}
		if !strings.Contains(result, bold) {
			t.Errorf("Markdown(%q) should contain bold ANSI code", tt.input)
		}
		// Should not contain the # prefix
		if strings.Contains(result, "# ") {
			t.Errorf("Markdown(%q) should strip # prefix, got %q", tt.input, result)
		}
	}
}

func TestMarkdown_Bold(t *testing.T) {
	result := Markdown("This is **bold** text")
	if !strings.Contains(result, bold+"bold"+reset) {
		t.Errorf("should render bold, got %q", result)
	}
}

func TestMarkdown_InlineCode(t *testing.T) {
	result := Markdown("Use `go build` here")
	if !strings.Contains(result, cyan) {
		t.Errorf("should render inline code with cyan, got %q", result)
	}
}

func TestMarkdown_CodeBlock(t *testing.T) {
	input := "```go\nfmt.Println(\"hello\")\n```"
	result := Markdown(input)
	if !strings.Contains(result, "go") {
		t.Errorf("should show language, got %q", result)
	}
	if !strings.Contains(result, "┌") {
		t.Errorf("should have top border, got %q", result)
	}
	if !strings.Contains(result, "└") {
		t.Errorf("should have bottom border, got %q", result)
	}
	if !strings.Contains(result, cyan) {
		t.Errorf("code content should be cyan, got %q", result)
	}
}

func TestMarkdown_CodeBlockNoLang(t *testing.T) {
	input := "```\nsome code\n```"
	result := Markdown(input)
	if !strings.Contains(result, "┌") {
		t.Errorf("should have top border, got %q", result)
	}
}

func TestMarkdown_ListItems(t *testing.T) {
	result := Markdown("- item one\n- item two")
	if !strings.Contains(result, "•") {
		t.Errorf("should render bullet points, got %q", result)
	}
	if strings.Count(result, "•") != 2 {
		t.Errorf("should have 2 bullets, got %q", result)
	}
}

func TestMarkdown_StarListItems(t *testing.T) {
	result := Markdown("* item one\n* item two")
	if strings.Count(result, "•") != 2 {
		t.Errorf("should have 2 bullets, got %q", result)
	}
}

func TestMarkdown_PlainText(t *testing.T) {
	input := "Just plain text without any markdown"
	result := Markdown(input)
	if result != input {
		t.Errorf("plain text should pass through unchanged, got %q", result)
	}
}

func TestMarkdown_EmptyString(t *testing.T) {
	result := Markdown("")
	if result != "" {
		t.Errorf("empty input should return empty, got %q", result)
	}
}

func TestMarkdown_MixedContent(t *testing.T) {
	input := "# Hello\n\nThis is **bold** and `code`.\n\n- item 1\n- item 2\n\n```go\nfmt.Println()\n```"
	result := Markdown(input)
	if !strings.Contains(result, "Hello") {
		t.Error("should contain header text")
	}
	if !strings.Contains(result, "•") {
		t.Error("should contain bullets")
	}
	if !strings.Contains(result, "┌") {
		t.Error("should contain code block")
	}
}
