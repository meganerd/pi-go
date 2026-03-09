// Package tool defines the tool interface and common types for pi-go tools.
package tool

import (
	"context"
	"encoding/json"
)

// Tool is the interface that all pi-go tools must implement.
type Tool interface {
	// Name returns the tool's identifier used in LLM function calling.
	Name() string

	// Description returns a human-readable description for the LLM.
	Description() string

	// Schema returns the JSON Schema for the tool's input parameters.
	Schema() json.RawMessage

	// Execute runs the tool with the given JSON input and returns a result.
	Execute(ctx context.Context, input json.RawMessage) (*Result, error)
}

// Result represents the output of a tool execution.
type Result struct {
	// Output is the main text output of the tool.
	Output string `json:"output"`

	// Truncated indicates whether the output was cut short.
	Truncated bool `json:"truncated,omitempty"`

	// TempFile is the path to a file containing the full output when truncated.
	TempFile string `json:"temp_file,omitempty"`

	// Lines is the total number of lines in the original output.
	Lines int `json:"lines,omitempty"`

	// Bytes is the total byte count of the original output.
	Bytes int `json:"bytes,omitempty"`
}

// FileSystem abstracts filesystem operations for testability and remote backends.
type FileSystem interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm uint32) error
	Stat(path string) (FileInfo, error)
	MkdirAll(path string, perm uint32) error
	Glob(pattern string) ([]string, error)
	ReadDir(path string) ([]DirEntry, error)
	Remove(path string) error
}

// FileInfo holds metadata about a file.
type FileInfo struct {
	Name  string
	Size  int64
	IsDir bool
	Mode  uint32
}

// DirEntry represents a directory entry.
type DirEntry struct {
	Name  string
	IsDir bool
}

// OSFileSystem implements FileSystem using the real OS filesystem.
type OSFileSystem struct{}
