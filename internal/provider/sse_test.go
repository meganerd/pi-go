package provider

import (
	"strings"
	"testing"
)

func TestParseSSE_SingleEvent(t *testing.T) {
	input := "event: message\ndata: hello world\n\n"
	ch := ParseSSE(strings.NewReader(input))

	event := <-ch
	if event.Event != "message" {
		t.Errorf("event = %q, want message", event.Event)
	}
	if event.Data != "hello world" {
		t.Errorf("data = %q, want hello world", event.Data)
	}

	// Channel should be closed
	if _, ok := <-ch; ok {
		t.Error("channel should be closed after all events")
	}
}

func TestParseSSE_MultipleEvents(t *testing.T) {
	input := "event: start\ndata: first\n\nevent: delta\ndata: second\n\n"
	ch := ParseSSE(strings.NewReader(input))

	events := make([]SSEEvent, 0)
	for e := range ch {
		events = append(events, e)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Data != "first" {
		t.Errorf("first event data = %q", events[0].Data)
	}
	if events[1].Data != "second" {
		t.Errorf("second event data = %q", events[1].Data)
	}
}

func TestParseSSE_MultiLineData(t *testing.T) {
	input := "data: line1\ndata: line2\n\n"
	ch := ParseSSE(strings.NewReader(input))

	event := <-ch
	if event.Data != "line1\nline2" {
		t.Errorf("multi-line data = %q, want line1\\nline2", event.Data)
	}
}

func TestParseSSE_NoTrailingNewline(t *testing.T) {
	input := "data: final"
	ch := ParseSSE(strings.NewReader(input))

	event := <-ch
	if event.Data != "final" {
		t.Errorf("data = %q, want final", event.Data)
	}
}

func TestParseSSE_EmptyInput(t *testing.T) {
	ch := ParseSSE(strings.NewReader(""))

	if _, ok := <-ch; ok {
		t.Error("empty input should produce no events")
	}
}

func TestParseSSE_DataOnly(t *testing.T) {
	input := "data: no event field\n\n"
	ch := ParseSSE(strings.NewReader(input))

	event := <-ch
	if event.Event != "" {
		t.Errorf("event should be empty, got %q", event.Event)
	}
	if event.Data != "no event field" {
		t.Errorf("data = %q", event.Data)
	}
}
