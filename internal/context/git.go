package context

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// GitContext returns git repository information for system prompt injection.
// Returns empty string if dir is not inside a git repo.
func GitContext(dir string) string {
	// Verify we're in a git repo
	root := gitCmd(dir, "rev-parse", "--show-toplevel")
	if root == "" {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\n## Git Context\n\n")
	fmt.Fprintf(&b, "Repository: %s\n", root)

	if branch := gitCmd(dir, "branch", "--show-current"); branch != "" {
		fmt.Fprintf(&b, "Branch: %s\n", branch)
	}

	if log := gitCmd(dir, "log", "--oneline", "-5"); log != "" {
		b.WriteString("\nRecent commits:\n")
		for _, line := range strings.Split(log, "\n") {
			if line != "" {
				fmt.Fprintf(&b, "  %s\n", line)
			}
		}
	}

	if diff := gitCmd(dir, "diff", "--stat"); diff != "" {
		b.WriteString("\nWorking tree changes:\n")
		for _, line := range strings.Split(diff, "\n") {
			if line != "" {
				fmt.Fprintf(&b, "  %s\n", line)
			}
		}
	}

	return b.String()
}

func gitCmd(dir string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
