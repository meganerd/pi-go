package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/meganerd/pi-go/internal/message"
)

func TestManager_CreateAndList(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// No sessions initially
	sessions, err := mgr.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}

	// Create a session
	store, info, err := mgr.Create("/home/user/project")
	if err != nil {
		t.Fatal(err)
	}
	if info.ID == "" {
		t.Error("session ID is empty")
	}
	if info.Path != "/home/user/project" {
		t.Errorf("session path: got %q, want %q", info.Path, "/home/user/project")
	}

	// Write a message so the file exists with content
	msg := message.Message{Role: message.RoleUser, Content: "hello"}
	store.Append(&msg)
	store.Close()

	// List should now show 1 session
	sessions, err = mgr.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != info.ID {
		t.Errorf("listed session ID: got %q, want %q", sessions[0].ID, info.ID)
	}
}

func TestManager_Open(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// Create and populate a session
	store1, info, _ := mgr.Create("/home/user/project")
	msg := message.Message{Role: message.RoleUser, Content: "persisted"}
	store1.Append(&msg)
	store1.Close()

	// Open the session
	store2, err := mgr.Open(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer store2.Close()

	history, err := store2.Messages()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 message, got %d", len(history))
	}
	if history[0].Content != "persisted" {
		t.Errorf("message content: got %q, want %q", history[0].Content, "persisted")
	}
}

func TestManager_ForPath(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// Create two sessions for different paths
	_, info1, _ := mgr.Create("/home/user/project-a")
	s1, _ := mgr.Open(info1.ID)
	msg := message.Message{Role: message.RoleUser, Content: "a"}
	s1.Append(&msg)
	s1.Close()

	_, info2, _ := mgr.Create("/home/user/project-b")
	s2, _ := mgr.Open(info2.ID)
	msg2 := message.Message{Role: message.RoleUser, Content: "b"}
	s2.Append(&msg2)
	s2.Close()

	// ForPath should find the session for project-a
	sessions, err := mgr.ForPath("/home/user/project-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session for project-a, got %d", len(sessions))
	}
	if sessions[0].ID != info1.ID {
		t.Errorf("got session %q, want %q", sessions[0].ID, info1.ID)
	}
}

func TestManager_SessionDirCreated(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "sessions")
	mgr := NewManager(dir)

	_, _, err := mgr.Create("/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	// Verify directory was created
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("session dir not created: %v", err)
	}
	if !fi.IsDir() {
		t.Error("session dir is not a directory")
	}
}
