// Package render provides terminal markdown rendering with ANSI escape codes.
package render

import (
	"regexp"
	"strings"
)

// ANSI escape codes for terminal styling.
const (
	reset     = "\033[0m"
	bold      = "\033[1m"
	dim       = "\033[2m"
	italic    = "\033[3m"
	underline = "\033[4m"
	cyan      = "\033[36m"
	green     = "\033[32m"
	yellow    = "\033[33m"
	magenta   = "\033[35m"
)

var (
	// Inline patterns — order matters (bold before italic).
	boldRe       = regexp.MustCompile(`\*\*(.+?)\*\*`)
	italicRe     = regexp.MustCompile(`(?:^|[^*])\*([^*]+?)\*(?:[^*]|$)`)
	inlineCodeRe = regexp.MustCompile("`([^`]+)`")
)

// Markdown renders a markdown string with ANSI terminal formatting.
// Handles headers, code blocks, bold, italic, inline code, and lists.
func Markdown(input string) string {
	lines := strings.Split(input, "\n")
	var out []string
	inCodeBlock := false
	codeLang := ""

	for _, line := range lines {
		// Code block toggle
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if !inCodeBlock {
				inCodeBlock = true
				codeLang = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "```"))
				if codeLang != "" {
					out = append(out, dim+"┌─ "+codeLang+" "+strings.Repeat("─", 40)+reset)
				} else {
					out = append(out, dim+"┌"+strings.Repeat("─", 44)+reset)
				}
				continue
			}
			inCodeBlock = false
			out = append(out, dim+"└"+strings.Repeat("─", 44)+reset)
			codeLang = ""
			continue
		}

		if inCodeBlock {
			out = append(out, dim+"│ "+reset+cyan+line+reset)
			continue
		}

		// Headers
		if strings.HasPrefix(line, "### ") {
			out = append(out, bold+yellow+line[4:]+reset)
			continue
		}
		if strings.HasPrefix(line, "## ") {
			out = append(out, bold+yellow+line[3:]+reset)
			continue
		}
		if strings.HasPrefix(line, "# ") {
			out = append(out, bold+underline+yellow+line[2:]+reset)
			continue
		}

		// List items
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			indent := line[:len(line)-len(trimmed)]
			content := renderInline(trimmed[2:])
			out = append(out, indent+green+"• "+reset+content)
			continue
		}

		// Numbered lists
		if matched, _ := regexp.MatchString(`^\s*\d+\.\s`, line); matched {
			out = append(out, renderInline(line))
			continue
		}

		// Regular line with inline formatting
		out = append(out, renderInline(line))
	}

	return strings.Join(out, "\n")
}

func renderInline(line string) string {
	// Inline code first (protect from bold/italic processing)
	line = inlineCodeRe.ReplaceAllString(line, cyan+"`$1`"+reset)

	// Bold before italic
	line = boldRe.ReplaceAllString(line, bold+"$1"+reset)

	// Italic — simple replacement avoiding bold markers
	line = italicRe.ReplaceAllStringFunc(line, func(match string) string {
		// Extract the content between single asterisks
		idx := strings.Index(match, "*")
		lastIdx := strings.LastIndex(match, "*")
		if idx == lastIdx || idx < 0 {
			return match
		}
		prefix := match[:idx]
		content := match[idx+1 : lastIdx]
		suffix := match[lastIdx+1:]
		return prefix + italic + content + reset + suffix
	})

	return line
}
