// Package context discovers and loads project context files for system prompt injection.
package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// contextFileNames lists filenames to search for, in priority order.
// The first found file wins per directory level.
var contextFileNames = []string{
	"CLAUDE.md",
	".pi-go.md",
	"PI-GO.md",
}

// ProjectContext holds discovered project context.
type ProjectContext struct {
	Files []ContextFile
}

// ContextFile represents a single context file.
type ContextFile struct {
	Path    string
	Content string
}

// Discover searches for project context files starting from dir and walking
// up to the filesystem root. Returns files found from most specific (cwd)
// to least specific (parent dirs).
func Discover(dir string) (*ProjectContext, error) {
	ctx := &ProjectContext{}
	seen := make(map[string]bool)

	dir, err := filepath.Abs(dir)
	if err != nil {
		return ctx, err
	}

	for {
		for _, name := range contextFileNames {
			path := filepath.Join(dir, name)
			if seen[path] {
				continue
			}
			seen[path] = true

			data, err := os.ReadFile(path)
			if err != nil {
				continue // file doesn't exist, try next
			}
			content := strings.TrimSpace(string(data))
			if content != "" {
				ctx.Files = append(ctx.Files, ContextFile{
					Path:    path,
					Content: content,
				})
			}
			break // only take first match per directory
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached root
		}
		dir = parent
	}

	return ctx, nil
}

// ForSystemPrompt formats discovered context files for injection into the system prompt.
func (p *ProjectContext) ForSystemPrompt() string {
	if len(p.Files) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\n## Project Context\n\n")
	b.WriteString("The following project context files were found and should guide your behavior:\n\n")

	for _, f := range p.Files {
		fmt.Fprintf(&b, "### %s\n\n%s\n\n", f.Path, f.Content)
	}

	return b.String()
}
