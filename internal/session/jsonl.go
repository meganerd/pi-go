package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/meganerd/pi-go/internal/message"
)

// JSONLStore implements Store using an append-only JSONL file with tree branching.
type JSONLStore struct {
	path    string
	file    *os.File
	entries map[string]*message.Message
	leaf    string
	counter int
	mu      sync.Mutex
}

// NewJSONLStore opens or creates a JSONL session file.
func NewJSONLStore(path string) (*JSONLStore, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("open session file: %w", err)
	}

	s := &JSONLStore{
		path:    path,
		file:    f,
		entries: make(map[string]*message.Message),
	}

	// Load existing entries
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var msg message.Message
		if err := json.Unmarshal(line, &msg); err != nil {
			// Skip partial/corrupt lines (crash recovery)
			continue
		}
		stored := msg
		s.entries[msg.ID] = &stored
		s.leaf = msg.ID
	}

	return s, nil
}

func (s *JSONLStore) nextID() string {
	s.counter++
	return fmt.Sprintf("%d_%04d", time.Now().UnixMicro(), s.counter)
}

// Append adds a message to the session, setting its ID and ParentID.
func (s *JSONLStore) Append(msg *message.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	msg.ID = s.nextID()
	msg.ParentID = s.leaf

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	data = append(data, '\n')

	if _, err := s.file.Write(data); err != nil {
		return fmt.Errorf("write message: %w", err)
	}
	if err := s.file.Sync(); err != nil {
		return fmt.Errorf("sync session file: %w", err)
	}

	stored := *msg
	s.entries[msg.ID] = &stored
	s.leaf = msg.ID

	return nil
}

// Messages returns the ordered history from root to current leaf.
func (s *JSONLStore) Messages() ([]message.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.leaf == "" {
		return nil, nil
	}

	// Walk from leaf to root
	var path []message.Message
	current := s.leaf
	for current != "" {
		msg, ok := s.entries[current]
		if !ok {
			return nil, fmt.Errorf("broken parent chain: missing message %q", current)
		}
		path = append(path, *msg)
		current = msg.ParentID
	}

	// Reverse to get root-to-leaf order
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	return path, nil
}

// Branch sets the active leaf to the given message ID.
func (s *JSONLStore) Branch(fromID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.entries[fromID]; !ok {
		return fmt.Errorf("branch: message %q not found", fromID)
	}
	s.leaf = fromID
	return nil
}

// Branches returns all message IDs that have more than one child.
func (s *JSONLStore) Branches() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	childCount := make(map[string]int)
	for _, msg := range s.entries {
		if msg.ParentID != "" {
			childCount[msg.ParentID]++
		}
	}

	var branches []string
	for id, count := range childCount {
		if count > 1 {
			branches = append(branches, id)
		}
	}
	return branches, nil
}

// Close flushes and closes the session file.
func (s *JSONLStore) Close() error {
	if s.file != nil {
		return s.file.Close()
	}
	return nil
}
