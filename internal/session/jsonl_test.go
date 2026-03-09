package session

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/meganerd/pi-go/internal/message"
)

func TestJSONLStore_AppendAndRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")

	store, err := NewJSONLStore(path)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Append three messages
	msgs := []message.Message{
		{Role: message.RoleUser, Content: "Hello"},
		{Role: message.RoleAssistant, Content: "Hi there!"},
		{Role: message.RoleUser, Content: "Read a file"},
	}
	for i := range msgs {
		if err := store.Append(&msgs[i]); err != nil {
			t.Fatalf("append %d failed: %v", i, err)
		}
	}

	// Read back
	history, err := store.Messages()
	if err != nil {
		t.Fatalf("messages failed: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(history))
	}
	if history[0].Content != "Hello" {
		t.Errorf("first message content: got %q, want %q", history[0].Content, "Hello")
	}
	if history[2].Content != "Read a file" {
		t.Errorf("third message content: got %q, want %q", history[2].Content, "Read a file")
	}

	// IDs should be set
	for i, m := range history {
		if m.ID == "" {
			t.Errorf("message %d has empty ID", i)
		}
	}

	// Parent chain: msg[1].ParentID == msg[0].ID, msg[2].ParentID == msg[1].ID
	if history[1].ParentID != history[0].ID {
		t.Errorf("msg[1].ParentID=%q, want %q", history[1].ParentID, history[0].ID)
	}
	if history[2].ParentID != history[1].ID {
		t.Errorf("msg[2].ParentID=%q, want %q", history[2].ParentID, history[1].ID)
	}

	store.Close()
}

func TestJSONLStore_ValidJSONPerLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")

	store, err := NewJSONLStore(path)
	if err != nil {
		t.Fatal(err)
	}

	msg := message.Message{Role: message.RoleUser, Content: "test"}
	store.Append(&msg)
	store.Close()

	// Read raw file and verify each line is valid JSON
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		var raw json.RawMessage
		if err := json.Unmarshal(scanner.Bytes(), &raw); err != nil {
			t.Errorf("line %d is not valid JSON: %v", lineNum, err)
		}
	}
	if lineNum == 0 {
		t.Error("file is empty")
	}
}

func TestJSONLStore_EmptySession(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")

	store, err := NewJSONLStore(path)
	if err != nil {
		t.Fatal(err)
	}

	history, err := store.Messages()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 0 {
		t.Errorf("expected 0 messages, got %d", len(history))
	}

	store.Close()
}

func TestJSONLStore_MessageOrdering(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")

	store, err := NewJSONLStore(path)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 5; i++ {
		msg := message.Message{Role: message.RoleUser, Content: string(rune('A' + i))}
		store.Append(&msg)
	}

	history, err := store.Messages()
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"A", "B", "C", "D", "E"}
	for i, want := range expected {
		if history[i].Content != want {
			t.Errorf("message %d: got %q, want %q", i, history[i].Content, want)
		}
	}

	store.Close()
}

func TestJSONLStore_Branch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")

	store, err := NewJSONLStore(path)
	if err != nil {
		t.Fatal(err)
	}

	// Create linear history: A -> B -> C
	msgA := message.Message{Role: message.RoleUser, Content: "A"}
	store.Append(&msgA)
	msgB := message.Message{Role: message.RoleAssistant, Content: "B"}
	store.Append(&msgB)
	msgC := message.Message{Role: message.RoleUser, Content: "C"}
	store.Append(&msgC)

	// Branch back to A — next message will be a sibling of B
	if err := store.Branch(msgA.ID); err != nil {
		t.Fatalf("branch failed: %v", err)
	}

	// Append D from branch point A
	msgD := message.Message{Role: message.RoleUser, Content: "D"}
	store.Append(&msgD)

	// History should be: A -> D (not A -> B -> C)
	history, err := store.Messages()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 messages on branch, got %d", len(history))
	}
	if history[0].Content != "A" || history[1].Content != "D" {
		t.Errorf("expected [A, D], got [%s, %s]", history[0].Content, history[1].Content)
	}

	// D's parent should be A
	if msgD.ParentID != msgA.ID {
		t.Errorf("D.ParentID=%q, want %q", msgD.ParentID, msgA.ID)
	}

	// Branches should include A (has children B and D)
	branches, err := store.Branches()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, b := range branches {
		if b == msgA.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("expected branch point %q in branches list %v", msgA.ID, branches)
	}

	store.Close()
}

func TestJSONLStore_ReopenAndResume(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")

	// First session: write two messages
	store1, _ := NewJSONLStore(path)
	msg1 := message.Message{Role: message.RoleUser, Content: "first"}
	store1.Append(&msg1)
	msg2 := message.Message{Role: message.RoleAssistant, Content: "second"}
	store1.Append(&msg2)
	store1.Close()

	// Reopen: should see existing messages
	store2, _ := NewJSONLStore(path)
	history, _ := store2.Messages()
	if len(history) != 2 {
		t.Fatalf("expected 2 messages after reopen, got %d", len(history))
	}

	// Append a third message — should continue the chain
	msg3 := message.Message{Role: message.RoleUser, Content: "third"}
	store2.Append(&msg3)

	history2, _ := store2.Messages()
	if len(history2) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(history2))
	}
	if history2[2].ParentID != history2[1].ID {
		t.Errorf("msg3.ParentID=%q, want %q", history2[2].ParentID, history2[1].ID)
	}

	store2.Close()
}
