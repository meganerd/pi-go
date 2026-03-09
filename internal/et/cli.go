package et

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CLIDelegator implements Delegator by shelling out to the et binary.
type CLIDelegator struct {
	binary     string // path to et binary
	configPath string // default config path
	outputDir  string // default output directory
}

// NewCLIDelegator creates a new CLI-based et delegator.
// If binary is empty, it searches PATH for "et".
// If configPath is empty, defaults to ~/electrictown.yaml.
// If outputDir is empty, defaults to /tmp/pi-go-et.
func NewCLIDelegator(binary, configPath, outputDir string) *CLIDelegator {
	if binary == "" {
		binary = "et"
	}
	if configPath == "" {
		home, _ := os.UserHomeDir()
		configPath = filepath.Join(home, "electrictown.yaml")
	}
	if outputDir == "" {
		outputDir = "/tmp/pi-go-et"
	}
	return &CLIDelegator{
		binary:     binary,
		configPath: configPath,
		outputDir:  outputDir,
	}
}

// Available checks whether the et binary is installed and the config exists.
func (d *CLIDelegator) Available() bool {
	// Check binary
	_, err := exec.LookPath(d.binary)
	if err != nil {
		return false
	}
	// Check config
	_, err = os.Stat(d.configPath)
	return err == nil
}

// Delegate sends a task to et for execution.
func (d *CLIDelegator) Delegate(ctx context.Context, task string, opts *Options) (*Result, error) {
	if opts == nil {
		opts = &Options{}
	}

	configPath := d.configPath
	if opts.ConfigPath != "" {
		configPath = opts.ConfigPath
	}

	outputDir := d.outputDir
	if opts.OutputDir != "" {
		outputDir = opts.OutputDir
	}

	// Ensure output dir exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	// Build command
	args := []string{
		"run",
		"--config", configPath,
		"--output-dir", outputDir,
	}
	if opts.Iterate {
		args = append(args, "--iterate")
	}
	args = append(args, opts.ExtraArgs...)
	args = append(args, task)

	cmd := exec.CommandContext(ctx, d.binary, args...)
	cmd.Env = os.Environ()

	// Capture output
	var output strings.Builder
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout // merge stderr into stdout

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start et: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		output.WriteString(scanner.Text())
		output.WriteByte('\n')
	}

	exitCode := 0
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("wait et: %w", err)
		}
	}

	// Find output files
	files := findOutputFiles(outputDir)

	return &Result{
		Output:   output.String(),
		Files:    files,
		ExitCode: exitCode,
	}, nil
}

// findOutputFiles returns all files in the output directory.
func findOutputFiles(dir string) []string {
	var files []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files
}
