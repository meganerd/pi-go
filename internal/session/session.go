// Package session defines the session persistence interface for pi-go.
package session

import (
	"github.com/meganerd/pi-go/internal/message"
)

// Store abstracts session persistence.
type Store interface {
	// Append adds a message to the session log.
	Append(msg message.Message) error

	// Messages returns the ordered message history for the current branch.
	Messages() ([]message.Message, error)

	// Branch creates a new branch at the given message ID.
	Branch(fromID string) error

	// Branches returns the list of branch points.
	Branches() ([]string, error)

	// Close flushes and closes the session store.
	Close() error
}

// Info holds metadata about a session.
type Info struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Path      string `json:"path"`
	Messages  int    `json:"messages"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
