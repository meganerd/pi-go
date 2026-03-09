package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Manager handles session discovery, creation, and listing.
type Manager struct {
	dir string
}

// NewManager creates a session manager for the given directory.
func NewManager(dir string) *Manager {
	return &Manager{dir: dir}
}

// Create creates a new session associated with the given working directory path.
func (m *Manager) Create(workPath string) (Store, *Info, error) {
	if err := os.MkdirAll(m.dir, 0755); err != nil {
		return nil, nil, fmt.Errorf("create session dir: %w", err)
	}

	id := fmt.Sprintf("%d", time.Now().UnixMicro())
	now := time.Now().UTC().Format(time.RFC3339)

	info := &Info{
		ID:        id,
		Path:      workPath,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Write metadata file
	metaPath := filepath.Join(m.dir, id+".meta.json")
	metaData, err := json.Marshal(info)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal session info: %w", err)
	}
	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return nil, nil, fmt.Errorf("write session metadata: %w", err)
	}

	// Create JSONL store
	storePath := filepath.Join(m.dir, id+".jsonl")
	store, err := NewJSONLStore(storePath)
	if err != nil {
		return nil, nil, err
	}

	return store, info, nil
}

// List returns all sessions in the session directory.
func (m *Manager) List() ([]Info, error) {
	entries, err := filepath.Glob(filepath.Join(m.dir, "*.meta.json"))
	if err != nil {
		return nil, fmt.Errorf("glob session files: %w", err)
	}

	var sessions []Info
	for _, metaPath := range entries {
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var info Info
		if err := json.Unmarshal(data, &info); err != nil {
			continue
		}
		sessions = append(sessions, info)
	}

	return sessions, nil
}

// Open opens an existing session by ID.
func (m *Manager) Open(id string) (Store, error) {
	storePath := filepath.Join(m.dir, id+".jsonl")
	if _, err := os.Stat(storePath); err != nil {
		return nil, fmt.Errorf("session %q not found: %w", id, err)
	}
	return NewJSONLStore(storePath)
}

// ForPath returns sessions associated with the given working directory path.
func (m *Manager) ForPath(workPath string) ([]Info, error) {
	all, err := m.List()
	if err != nil {
		return nil, err
	}

	var matched []Info
	for _, info := range all {
		if strings.EqualFold(info.Path, workPath) {
			matched = append(matched, info)
		}
	}
	return matched, nil
}
