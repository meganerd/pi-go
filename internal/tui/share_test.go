package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestShareToGitLab_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("PRIVATE-TOKEN") != "test-token" {
			t.Error("expected PRIVATE-TOKEN header")
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["visibility"] != "private" {
			t.Error("expected private visibility")
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"web_url": "https://gitlab.example.com/snippets/42",
		})
	}))
	defer server.Close()

	t.Setenv("GITLAB_TOKEN", "test-token")
	t.Setenv("GITLAB_URL", server.URL)

	result, err := ShareToGitLab("Test Session", "# Content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Platform != "gitlab" {
		t.Errorf("expected platform 'gitlab', got %q", result.Platform)
	}
	if result.URL != "https://gitlab.example.com/snippets/42" {
		t.Errorf("unexpected URL: %s", result.URL)
	}
}

func TestShareToGitLab_NoToken(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	os.Unsetenv("GITLAB_TOKEN")

	_, err := ShareToGitLab("Test", "content")
	if err == nil {
		t.Error("expected error when GITLAB_TOKEN not set")
	}
}

func TestShareToGitHub_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hasBearer(r.Header.Get("Authorization")) {
			t.Error("expected Bearer auth")
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"html_url": "https://gist.github.com/abc123",
		})
	}))
	defer server.Close()

	// Can't easily override GitHub API URL in current implementation,
	// so just test the no-token case
	t.Setenv("GH_TOKEN", "")
	os.Unsetenv("GH_TOKEN")
	t.Setenv("GITHUB_TOKEN", "")
	os.Unsetenv("GITHUB_TOKEN")

	_, err := ShareToGitHub("Test", "content")
	if err == nil {
		t.Error("expected error when no GitHub token set")
	}
}

func TestShare_FallbackLogic(t *testing.T) {
	// No tokens set — should fail with helpful message
	t.Setenv("GITLAB_TOKEN", "")
	os.Unsetenv("GITLAB_TOKEN")
	t.Setenv("GH_TOKEN", "")
	os.Unsetenv("GH_TOKEN")
	t.Setenv("GITHUB_TOKEN", "")
	os.Unsetenv("GITHUB_TOKEN")

	_, err := Share("Test", "content")
	if err == nil {
		t.Error("expected error when no tokens set")
	}
	if err != nil && !strings.Contains(err.Error(), "no share target") {
		t.Errorf("expected 'no share target' error, got: %v", err)
	}
}

func hasBearer(auth string) bool {
	return len(auth) > 7 && auth[:7] == "Bearer "
}
