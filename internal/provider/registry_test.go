package provider

import (
	"testing"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()

	called := false
	reg.Register("test", func(apiKey string, baseURL string) Provider {
		called = true
		return nil
	})

	names := reg.Names()
	if len(names) != 1 || names[0] != "test" {
		t.Errorf("expected [test], got %v", names)
	}

	reg.Get("test", "key", "url")
	if !called {
		t.Error("constructor was not called")
	}
}

func TestRegistry_GetUnknown(t *testing.T) {
	reg := NewRegistry()
	p := reg.Get("nonexistent", "key", "url")
	if p != nil {
		t.Error("expected nil for unknown provider")
	}
}
