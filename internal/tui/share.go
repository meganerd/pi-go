package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// ShareResult holds the result of a share operation.
type ShareResult struct {
	URL      string
	Platform string // "gitlab" or "github"
}

// ShareToGitLab creates a private snippet on a GitLab instance.
// Uses GITLAB_TOKEN for auth and GITLAB_URL for the instance URL
// (defaults to gitlab.lan.meganerd.ca).
func ShareToGitLab(title, content string) (*ShareResult, error) {
	token := os.Getenv("GITLAB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITLAB_TOKEN not set")
	}

	baseURL := os.Getenv("GITLAB_URL")
	if baseURL == "" {
		baseURL = "https://gitlab.lan.meganerd.ca"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	payload := map[string]interface{}{
		"title":      title,
		"content":    content,
		"file_name":  "session.md",
		"visibility": "private",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal snippet: %w", err)
	}

	url := baseURL + "/api/v4/snippets"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("PRIVATE-TOKEN", token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("GitLab API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		WebURL string `json:"web_url"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &ShareResult{URL: result.WebURL, Platform: "gitlab"}, nil
}

// ShareToGitHub creates a private gist on GitHub.
// Uses GH_TOKEN or GITHUB_TOKEN for auth.
func ShareToGitHub(title, content string) (*ShareResult, error) {
	token := os.Getenv("GH_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		return nil, fmt.Errorf("GH_TOKEN or GITHUB_TOKEN not set")
	}

	payload := map[string]interface{}{
		"description": title,
		"public":      false,
		"files": map[string]interface{}{
			"session.md": map[string]string{
				"content": content,
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal gist: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.github.com/gists", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("GitHub API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &ShareResult{URL: result.HTMLURL, Platform: "github"}, nil
}

// Share tries GitLab first (local instance), then falls back to GitHub.
func Share(title, content string) (*ShareResult, error) {
	// Try GitLab first (local instance)
	if os.Getenv("GITLAB_TOKEN") != "" {
		result, err := ShareToGitLab(title, content)
		if err == nil {
			return result, nil
		}
		// Fall through to GitHub
	}

	// Try GitHub
	result, err := ShareToGitHub(title, content)
	if err != nil {
		return nil, fmt.Errorf("no share target available: set GITLAB_TOKEN for GitLab snippets or GH_TOKEN for GitHub gists")
	}
	return result, nil
}
