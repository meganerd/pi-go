// Package session defines the session persistence interface for pi-go.
package session

import (
	"github.com/meganerd/pi-go/internal/message"
)

// Store abstracts session persistence.
type Store interface {
	// Append adds a message to the session log, setting its ID and ParentID.
	Append(msg *message.Message) error

	// Messages returns the ordered message history from root to current leaf.
	Messages() ([]message.Message, error)

	// Branch sets the active leaf to the given message ID.
	// Subsequent appends will parent from this message.
	Branch(fromID string) error

	// Branches returns the list of all message IDs that have multiple children.
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
